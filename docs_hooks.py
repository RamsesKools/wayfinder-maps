import logging
import os
import re
import shutil
import subprocess

from markdown import Extension
from markdown.preprocessors import Preprocessor
from mkdocs.exceptions import PluginError
from mkdocs.structure.files import File

log = logging.getLogger("mkdocs.hooks.docs_hooks")

# The full-screen demo viewer at /demo-fullscreen/ (embedded by docs/demo.md): the fixture
# map exported as a static site on every build, so it always matches the viewer and the map.
DEMO_MAP = "internal/wayfinder/testdata/implement-demo"
DEMO_DIR = "demo-fullscreen"

# README.md links from the repo root (docs/images/x.png); docs/index.md pulls it in
# via snippets, so the same links must resolve from inside docs/.
README_DOCS_LINK = re.compile(r'(\]\(|src=")docs/')


class ReadmeLinks(Preprocessor):
    def run(self, lines):
        return [README_DOCS_LINK.sub(r"\1", line) for line in lines]


class ReadmeLinksExtension(Extension):
    def extendMarkdown(self, md):
        # Below pymdownx.snippets (32), so it sees the included README.
        md.preprocessors.register(ReadmeLinks(md), "readme_links", 31)


# The header compass reuses the viewer's own dial, so the docs publish its modules as they are.
VIEWER_JS = ["compass.js", "direction.js", "util.js"]


def on_files(files, config):
    web = os.path.join(os.path.dirname(config.config_file_path), "cmd/wayfinder-maps/web/js")
    for name in VIEWER_JS:
        files.append(File.generated(config, "assets/viewer/" + name, abs_src_path=os.path.join(web, name)))
    return files


def on_config(config):
    config.markdown_extensions.append(ReadmeLinksExtension())
    return config


def on_post_build(config):
    root = os.path.dirname(config.config_file_path)
    if not shutil.which("go"):
        # A warning, so strict builds (CI, release) fail while a plain local build still works.
        log.warning("go not found: /%s/ was not generated", DEMO_DIR)
        return
    out = os.path.join(config.site_dir, DEMO_DIR)
    result = subprocess.run(
        ["go", "run", "./cmd/wayfinder-maps", "export", DEMO_MAP, out],
        cwd=root,
        env={**os.environ, "CGO_ENABLED": "0"},
        capture_output=True,
        text=True,
    )
    if result.returncode != 0:
        raise PluginError("demo export failed:\n" + result.stderr)
    log.info("Exported the demo map to %s", out)

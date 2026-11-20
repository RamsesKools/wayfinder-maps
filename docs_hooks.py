import logging
import os
import re
import shutil
import subprocess

from markdown import Extension
from markdown.preprocessors import Preprocessor
from mkdocs.exceptions import PluginError

log = logging.getLogger("mkdocs.hooks.docs_hooks")

# The live demo at /demo/: the fixture map exported as a static site on every build,
# so it always matches the viewer and the map in this checkout.
DEMO_MAP = "internal/wayfinder/testdata/implement-demo"

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


def on_config(config):
    config.markdown_extensions.append(ReadmeLinksExtension())
    return config


def on_post_build(config):
    root = os.path.dirname(config.config_file_path)
    if not shutil.which("go"):
        # A warning, so strict builds (CI, release) fail while a plain local build still works.
        log.warning("go not found: /demo/ was not generated")
        return
    out = os.path.join(config.site_dir, "demo")
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

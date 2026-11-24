import logging
import os
import shutil
import subprocess

from mkdocs.exceptions import PluginError
from mkdocs.structure.files import File

log = logging.getLogger("mkdocs.hooks.docs_hooks")

# The full-screen demo viewer at /demo-fullscreen/ (embedded by docs/demo.md): the fixture
# map exported as a static site on every build, so it always matches the viewer and the map.
DEMO_MAP = "internal/wayfinder/testdata/implement-demo"
DEMO_DIR = "demo-fullscreen"

# The header compass reuses the viewer's own dial, so the docs publish its modules as they are.
VIEWER_JS = ["compass.js", "direction.js", "util.js"]


def on_files(files, config):
    web = os.path.join(os.path.dirname(config.config_file_path), "cmd/wayfinder-maps/web/js")
    for name in VIEWER_JS:
        files.append(File.generated(config, "assets/viewer/" + name, abs_src_path=os.path.join(web, name)))
    return files


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

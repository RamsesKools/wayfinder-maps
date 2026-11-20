import re

from markdown import Extension
from markdown.preprocessors import Preprocessor

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

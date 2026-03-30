from pathlib import Path
import re

from setuptools import setup


ROOT = Path(__file__).parent
INIT_FILE = ROOT / "agentmsg" / "__init__.py"
README = ROOT / "README.md"


def read_version() -> str:
    match = re.search(r'^__version__ = "([^"]+)"', INIT_FILE.read_text(encoding="utf-8"), re.MULTILINE)
    if not match:
        raise RuntimeError("Unable to find Python SDK version")
    return match.group(1)


setup(
    version=read_version(),
    long_description=README.read_text(encoding="utf-8"),
    long_description_content_type="text/markdown",
)

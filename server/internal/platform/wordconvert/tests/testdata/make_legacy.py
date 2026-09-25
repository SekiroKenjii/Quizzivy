import importlib.util
from pathlib import Path
import subprocess
import sys

spec = importlib.util.spec_from_file_location("converter", "/opt/quizzivy-convert.py")
converter = importlib.util.module_from_spec(spec)
spec.loader.exec_module(converter)

for directory in ("home", "tmp", "cache", "profile"):
    Path("/work", directory).mkdir(mode=0o700)

with subprocess.Popen(
    ["soffice", "--headless", "--norestore", "--nodefault", "--nologo",
     "-env:UserInstallation=file:///work/profile",
     "--accept=pipe,name=quizzivy_import;urp;StarOffice.ServiceManager"],
    stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL,
) as process:
    try:
        document = converter.load_document(converter.connect(process), Path("/input/source.docx"))
        try:
            settings = {"FilterName": "MS Word 97", "Overwrite": False}
            if len(sys.argv) > 1:
                settings["Password"] = "synthetic-password"
            document.storeToURL("file:///work/fixture.doc", converter.properties(**settings))
        finally:
            document.close(True)
    finally:
        process.terminate()
        process.wait(timeout=5)

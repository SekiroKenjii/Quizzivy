import json
import os
from pathlib import Path
import re
import subprocess
import sys
import time

import uno
import unohelper
from com.sun.star.beans import PropertyValue
from com.sun.star.task import XInteractionHandler


class AbortInteraction(unohelper.Base, XInteractionHandler):
    def handle(self, request):
        abort_type = uno.getTypeByName("com.sun.star.task.XInteractionAbort")
        for continuation in request.getContinuations():
            abort = continuation.queryInterface(abort_type)
            if abort is not None:
                abort.select()
                return


def properties(**values):
    result = []
    for name, value in values.items():
        prop = PropertyValue()
        prop.Name, prop.Value = name, value
        result.append(prop)
    return tuple(result)


def connect(process):
    local = uno.getComponentContext()
    resolver = local.ServiceManager.createInstanceWithContext(
        "com.sun.star.bridge.UnoUrlResolver", local
    )
    deadline = time.monotonic() + 15
    while time.monotonic() < deadline:
        if process.poll() is not None:
            raise RuntimeError("CONVERTER_START_FAILED")
        try:
            return resolver.resolve(
                "uno:pipe,name=quizzivy_import;urp;StarOffice.ComponentContext"
            )
        except Exception:
            time.sleep(0.1)
    raise RuntimeError("CONVERTER_START_TIMEOUT")


def load_document(context, source):
    desktop = context.ServiceManager.createInstanceWithContext(
        "com.sun.star.frame.Desktop", context
    )
    document = desktop.loadComponentFromURL(
        source.as_uri(),
        "_blank",
        0,
        properties(
            Hidden=True,
            ReadOnly=True,
            MacroExecutionMode=uno.Any("short", 0),
            UpdateDocMode=uno.Any("short", 0),
            RepairPackage=False,
            InteractionHandler=AbortInteraction(),
        ),
    )
    if document is None or not document.supportsService("com.sun.star.text.TextDocument"):
        raise RuntimeError("SOURCE_LOCKED_OR_UNSUPPORTED")
    return document


def render_pages(pdf, output):
    info = subprocess.run(
        ["pdfinfo", str(pdf)], check=True, capture_output=True, timeout=10
    ).stdout.decode("utf-8", errors="replace")
    match = re.search(r"^Pages:\s+(\d+)\s*$", info, re.MULTILINE)
    if match is None or not 1 <= int(match.group(1)) <= 60:
        raise RuntimeError("SOURCE_PAGE_LIMIT")
    count = int(match.group(1))
    subprocess.run(
        ["pdftoppm", "-png", "-scale-to", "1600", "-f", "1", "-l", str(count), str(pdf), str(output / "page")],
        check=True, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, timeout=60,
    )
    pages = sorted(output.glob("page-*.png"))
    if len(pages) != count:
        raise RuntimeError("RENDITION_INCOMPLETE")
    return [page.name for page in pages]


def convert(format_name, context, output):
    source = Path("/input/source." + format_name)
    if not source.is_file() or source.stat().st_size > 25 * 1024 * 1024:
        raise RuntimeError("SOURCE_INVALID")
    document = load_document(context, source)
    try:
        if format_name == "doc":
            document.storeToURL(
                (output / "normalized.docx").as_uri(),
                properties(FilterName="Office Open XML Text", Overwrite=False),
            )
        document.storeToURL(
            (output / "source.pdf").as_uri(),
            properties(
                FilterName="writer_pdf_Export",
                Overwrite=False,
                FilterData=uno.Any("[]com.sun.star.beans.PropertyValue", properties(
                    ExportFormFields=False, ExportNotes=False, ExportBookmarks=False,
                    IsSkipEmptyPages=False,
                )),
            ),
        )
    finally:
        document.close(True)
    return render_pages(output / "source.pdf", output)


def main():
    if len(sys.argv) != 2 or sys.argv[1] not in ("doc", "docx"):
        raise RuntimeError("SOURCE_FORMAT_UNSUPPORTED")
    os.umask(0o077)
    work = Path("/work")
    for directory in ("home", "tmp", "cache", "profile", "output"):
        (work / directory).mkdir(mode=0o700, exist_ok=False)
    version = subprocess.run(
        ["soffice", "--version"], check=True, capture_output=True, timeout=10
    ).stdout.decode("utf-8").strip()
    with subprocess.Popen(
        ["soffice", "--headless", "--nologo", "--nodefault", "--norestore", "--nofirststartwizard",
         "-env:UserInstallation=file:///work/profile", "--accept=pipe,name=quizzivy_import;urp;StarOffice.ServiceManager"],
        stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL,
    ) as process:
        try:
            pages = convert(sys.argv[1], connect(process), work / "output")
            files = list((work / "output").iterdir())
            if sum(path.stat().st_size for path in files) > 128 * 1024 * 1024:
                raise RuntimeError("ARTIFACT_BYTES_EXCEEDED")
            manifest = {
                "version": "libreoffice-rendition-v1", "renderer": version,
                "sourceFormat": sys.argv[1], "pages": pages,
                "normalized": "normalized.docx" if sys.argv[1] == "doc" else None,
                "pdf": "source.pdf", "findings": ["RENDERER_LAYOUT_REQUIRES_REVIEW"],
            }
            if sys.argv[1] == "doc":
                manifest["findings"].append("LEGACY_NORMALIZATION_REQUIRES_REVIEW")
            (work / "output" / "manifest.json").write_text(json.dumps(manifest), encoding="utf-8")
        finally:
            process.terminate()
            try:
                process.wait(timeout=5)
            except subprocess.TimeoutExpired:
                process.kill()
                process.wait()


if __name__ == "__main__":
    try:
        main()
    except Exception:
        print("CONVERSION_FAILED", file=sys.stderr)
        sys.exit(1)

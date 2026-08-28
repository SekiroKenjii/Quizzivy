#!/usr/bin/env python3
"""Structural assertions over api/openapi.yaml.

These are the invariants that must not regress, checked mechanically rather
than by review. T-0.13 ports them into the Vitest suite so they run alongside
everything else; until the web project exists this is where they live.

Run: python3 api/contract_check.py
"""
import sys
import pathlib
import yaml

SPEC = pathlib.Path(__file__).parent / "openapi.yaml"

# §13.5: these must never reach a student. `transcript` is the one exception,
# permitted on the result endpoint only, gated by showTranscriptAfterSubmit.
FORBIDDEN = ["isCorrect", "sampleAnswer", "acceptedAnswers", "transcript"]
TRANSCRIPT_ALLOWED_AT = {"/app/attempts/{id}/result"}

failures: list[str] = []
checks = 0


def check(ok: bool, label: str, detail: str = "") -> None:
    global checks
    checks += 1
    if not ok:
        failures.append(f"{label}{': ' + detail if detail else ''}")


def walk_refs(node, doc, seen: set[str], out: set[str]) -> None:
    """Collect every schema component name reachable from `node`."""
    if isinstance(node, dict):
        ref = node.get("$ref")
        if isinstance(ref, str) and ref.startswith("#/components/schemas/"):
            name = ref.rsplit("/", 1)[1]
            if name not in seen:
                seen.add(name)
                out.add(name)
                target = doc["components"]["schemas"].get(name)
                if target is not None:
                    walk_refs(target, doc, seen, out)
            return
        for v in node.values():
            walk_refs(v, doc, seen, out)
    elif isinstance(node, list):
        for v in node:
            walk_refs(v, doc, seen, out)


def property_names(node, doc, depth: int = 0) -> set[str]:
    """Every property name reachable from a schema, following $refs."""
    names: set[str] = set()
    if depth > 40 or not isinstance(node, (dict, list)):
        return names
    if isinstance(node, list):
        for v in node:
            names |= property_names(v, doc, depth + 1)
        return names
    ref = node.get("$ref")
    if isinstance(ref, str) and ref.startswith("#/components/schemas/"):
        target = doc["components"]["schemas"].get(ref.rsplit("/", 1)[1])
        return property_names(target, doc, depth + 1) if target else names
    props = node.get("properties")
    if isinstance(props, dict):
        names |= set(props.keys())
        for v in props.values():
            names |= property_names(v, doc, depth + 1)
    for key in ("items", "allOf", "oneOf", "anyOf", "additionalProperties", "not"):
        if key in node:
            names |= property_names(node[key], doc, depth + 1)
    return names


def main() -> int:
    doc = yaml.safe_load(SPEC.read_text())
    schemas = doc.get("components", {}).get("schemas", {})
    paths = doc.get("paths", {})

    # ---------------------------------------------------------- 1. no dangling refs
    all_refs: set[str] = set()

    def collect(n):
        if isinstance(n, dict):
            r = n.get("$ref")
            if isinstance(r, str):
                all_refs.add(r)
            for v in n.values():
                collect(v)
        elif isinstance(n, list):
            for v in n:
                collect(v)

    collect(doc)
    for ref in sorted(all_refs):
        if not ref.startswith("#/"):
            continue
        node = doc
        for part in ref[2:].split("/"):
            node = node.get(part) if isinstance(node, dict) else None
            if node is None:
                break
        check(node is not None, "dangling $ref", ref)

    # ------------------------------------------- 2. the student-payload boundary
    for path, item in paths.items():
        if not path.startswith("/app/"):
            continue
        for method, op in item.items():
            if method in ("parameters", "servers") or not isinstance(op, dict):
                continue
            for status, resp in (op.get("responses") or {}).items():
                if not str(status).startswith("2"):
                    continue
                schema = ((resp.get("content") or {}).get("application/json") or {}).get("schema")
                if schema is None:
                    continue
                names = property_names(schema, doc)
                for bad in FORBIDDEN:
                    if bad not in names:
                        continue
                    if bad == "transcript" and path in TRANSCRIPT_ALLOWED_AT:
                        continue
                    check(False, "student payload leak",
                          f"{method.upper()} {path} {status} exposes '{bad}'")
            check(True, f"student payload clean: {method.upper()} {path}")

    # StudentQuestion must be clean in isolation, whatever it is used by
    sq_names = property_names(schemas.get("StudentQuestion"), doc)
    for bad in FORBIDDEN:
        check(bad not in sq_names, "StudentQuestion leak", bad)

    # AdminQuestion must still carry the grading key -- if it stopped, grading
    # would silently break rather than loudly fail.
    aq_names = property_names(schemas.get("AdminQuestion"), doc)
    for needed in ("isCorrect", "sampleAnswer", "acceptedAnswers"):
        check(needed in aq_names, "AdminQuestion missing grading key", needed)

    # ------------------------------------ 3. every public operation is rate-limited
    for path, item in paths.items():
        for method, op in item.items():
            if method in ("parameters", "servers") or not isinstance(op, dict):
                continue
            sec = op.get("security")
            is_public = sec is not None and (sec == [] or any(s == {} for s in sec))
            if not is_public:
                continue
            label = f"{method.upper()} {path}"
            check("x-rate-limit" in op or path.endswith("/events"),
                  "public operation without x-rate-limit", label)
            check("public" in (op.get("tags") or []) or path.startswith("/app/"),
                  "public operation not tagged 'public'", label)
            check("429" in (op.get("responses") or {}) or path.endswith("/events"),
                  "public operation without a 429 response", label)

    # ------------------------------------------------ 4. operationIds are unique
    seen_ids: dict[str, str] = {}
    for path, item in paths.items():
        for method, op in item.items():
            if method in ("parameters", "servers") or not isinstance(op, dict):
                continue
            oid = op.get("operationId")
            check(bool(oid), "missing operationId", f"{method.upper()} {path}")
            if oid:
                check(oid not in seen_ids, "duplicate operationId",
                      f"{oid} ({seen_ids.get(oid)} and {method.upper()} {path})")
                seen_ids[oid] = f"{method.upper()} {path}"

    # ------------------------------------- 5. list responses share one envelope
    for path, item in paths.items():
        for method, op in item.items():
            if method != "get" or not isinstance(op, dict):
                continue
            if "cursor" not in str(op.get("parameters", "")):
                continue
            schema = (((op.get("responses") or {}).get("200") or {}).get("content") or {})
            schema = (schema.get("application/json") or {}).get("schema") or {}
            names = property_names(schema, doc)
            check("nextCursor" in names and "items" in names,
                  "paginated response missing the {items,nextCursor} envelope",
                  f"{method.upper()} {path}")

    # ------------------------------------------------------------------ report
    total_ops = sum(
        1 for it in paths.values() for m, o in it.items()
        if m not in ("parameters", "servers") and isinstance(o, dict)
    )
    print(f"api/openapi.yaml: {len(paths)} paths, {total_ops} operations, "
          f"{len(schemas)} schemas, {checks} assertions")
    if failures:
        print(f"\nFAILED ({len(failures)}):")
        for f in failures:
            print(f"  - {f}")
        return 1
    print("all contract assertions passed")
    return 0


if __name__ == "__main__":
    sys.exit(main())

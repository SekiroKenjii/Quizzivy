import Ajv2020, { type ValidateFunction } from "ajv/dist/2020";
import addFormats from "ajv-formats";
import { HttpResponse } from "msw";
import { loadSpec, type Json } from "./openapi";

/** Validates every mock response against the real contract before returning it. */

const spec = loadSpec();

const ajv = new Ajv2020({
  strict: false,
  allErrors: true,
});
addFormats(ajv);
ajv.addSchema(spec, "openapi");

const cache = new Map<string, ValidateFunction>();

function escapePointer(segment: string): string {
  return segment.replace(/~/g, "~0").replace(/\//g, "~1");
}

function responsePointer(path: string, method: string, status: number): string {
  const op = spec?.paths?.[path]?.[method] as Json;
  const ref = op?.responses?.[String(status)]?.$ref as string | undefined;
  if (ref?.startsWith("#/")) return `openapi${ref}`;
  return `openapi#/paths/${escapePointer(path)}/${method}/responses/${status}`;
}

function responseAt(path: string, method: string, status: number): Json {
  const op = spec?.paths?.[path]?.[method] as Json;
  const response = op?.responses?.[String(status)] as Json;
  const ref = response?.$ref as string | undefined;
  if (!ref?.startsWith("#/components/responses/")) return response;
  return spec?.components?.responses?.[
    ref.slice("#/components/responses/".length)
  ] as Json;
}

function validatorFor(path: string, method: string, status: number): ValidateFunction {
  const key = `${method} ${path} ${status}`;
  const cached = cache.get(key);
  if (cached) return cached;

  const pointer =
    responsePointer(path, method, status) +
    `/content/${escapePointer("application/json")}/schema`;

  const validate = ajv.compile({ $ref: pointer });
  cache.set(key, validate);
  return validate;
}

/** True when the contract actually defines a JSON body for this response. */
function hasJsonBody(path: string, method: string, status: number): boolean {
  return Boolean(
    responseAt(path, method, status)?.content?.["application/json"]?.schema,
  );
}

/**
 * MSW's `HttpResponse.json`, with the body checked against the contract first.
 *
 * `path` and `method` are the OpenAPI path template and method, not the request
 * URL — `/admin/tests/{id}`, not `/admin/tests/abc`.
 */
export function contractJson(
  path: string,
  method: string,
  status: number,
  body: unknown,
): Response {
  if (!hasJsonBody(path, method, status)) {
    throw new Error(
      `Mock declares ${method.toUpperCase()} ${path} -> ${status} with a JSON body, ` +
        `but api/openapi.yaml defines no such response. Fix the mock or the contract.`,
    );
  }

  const validate = validatorFor(path, method, status);
  if (!validate(body)) {
    const problems = (validate.errors ?? [])
      .map((e) => `  ${e.instancePath || "(root)"} ${e.message}`)
      .join("\n");
    throw new Error(
      `Mock response for ${method.toUpperCase()} ${path} (${status}) does not match ` +
        `api/openapi.yaml:\n${problems}\n\nBody: ${JSON.stringify(body, null, 2)}`,
    );
  }

  return HttpResponse.json(body as never, { status });
}

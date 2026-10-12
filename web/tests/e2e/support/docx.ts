import { crc32 } from "node:zlib";

const CONTENT_TYPES =
  '<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">' +
  '<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>' +
  '<Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>' +
  "</Types>";

const ROOT_RELATIONSHIPS =
  '<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">' +
  '<Relationship Id="r1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>' +
  "</Relationships>";

const escapeXml = (text: string) =>
  text.replaceAll("&", "&amp;").replaceAll("<", "&lt;").replaceAll(">", "&gt;");

function documentXml(paragraphs: readonly string[]): string {
  const body = paragraphs
    .map(
      (text) =>
        `<w:p><w:r><w:t xml:space="preserve">${escapeXml(text)}</w:t></w:r></w:p>`,
    )
    .join("");
  return (
    '<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">' +
    `<w:body>${body}</w:body></w:document>`
  );
}

interface Entry {
  name: Buffer;
  data: Buffer;
  crc: number;
  offset: number;
}

function localHeader(entry: Entry): Buffer {
  const header = Buffer.alloc(30);
  header.writeUInt32LE(0x04034b50, 0);
  header.writeUInt16LE(20, 4);
  header.writeUInt16LE(0x0800, 6);
  header.writeUInt32LE(entry.crc, 14);
  header.writeUInt32LE(entry.data.length, 18);
  header.writeUInt32LE(entry.data.length, 22);
  header.writeUInt16LE(entry.name.length, 26);
  return Buffer.concat([header, entry.name, entry.data]);
}

function centralHeader(entry: Entry): Buffer {
  const header = Buffer.alloc(46);
  header.writeUInt32LE(0x02014b50, 0);
  header.writeUInt16LE(20, 4);
  header.writeUInt16LE(20, 6);
  header.writeUInt16LE(0x0800, 8);
  header.writeUInt32LE(entry.crc, 16);
  header.writeUInt32LE(entry.data.length, 20);
  header.writeUInt32LE(entry.data.length, 24);
  header.writeUInt16LE(entry.name.length, 28);
  header.writeUInt32LE(entry.offset, 42);
  return Buffer.concat([header, entry.name]);
}

function zip(files: ReadonlyArray<readonly [string, string]>): Buffer {
  const entries: Entry[] = [];
  const locals: Buffer[] = [];
  let offset = 0;
  for (const [name, text] of files) {
    const data = Buffer.from(text, "utf8");
    const entry = { name: Buffer.from(name, "utf8"), data, crc: crc32(data), offset };
    const local = localHeader(entry);
    entries.push(entry);
    locals.push(local);
    offset += local.length;
  }
  const central = Buffer.concat(entries.map(centralHeader));
  const end = Buffer.alloc(22);
  end.writeUInt32LE(0x06054b50, 0);
  end.writeUInt16LE(entries.length, 8);
  end.writeUInt16LE(entries.length, 10);
  end.writeUInt32LE(central.length, 12);
  end.writeUInt32LE(offset, 16);
  return Buffer.concat([...locals, central, end]);
}

/**
 * wordDocument returns a minimal .docx whose body is one paragraph per entry
 * of `paragraphs`: the three parts the server's inspector and extractor need
 * and nothing else, so a test can write a paper as plain lines.
 */
export function wordDocument(paragraphs: readonly string[]): Buffer {
  return zip([
    ["[Content_Types].xml", CONTENT_TYPES],
    ["_rels/.rels", ROOT_RELATIONSHIPS],
    ["word/document.xml", documentXml(paragraphs)],
  ]);
}

export const DOCX_TYPE =
  "application/vnd.openxmlformats-officedocument.wordprocessingml.document";

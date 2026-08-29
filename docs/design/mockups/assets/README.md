# Deck assets

## `kit.css`

Hand-written component layer + a generated utility layer, in one file. The component layer is
the top of the file down to the `Tailwind-named utility subset` marker; everything below is
generated and should not be hand-edited — regenerate it instead.

Tokens are copied from `web/src/index.css`. Keep them identical; the deck's whole claim to
being trustworthy rests on it.

## `icons.js`

The lucide icons the sheets use, inlined as an SVG sprite and injected on load. `<use>` against
an external SVG file does not work over `file://`, which is why this is JavaScript rather than
an `.svg`.

Regenerate after adding an icon:

```bash
# 1. fetch the new icon (names follow lucide v1.x — `house`, `ellipsis`, `triangle-alert`, …)
curl -sfSL "https://unpkg.com/lucide-static@1.35.0/icons/<name>.svg" -o /tmp/icons/<name>.svg

# 2. rebuild the sprite from a directory of lucide SVGs
node -e '
  const {readdirSync,readFileSync,writeFileSync}=require("node:fs");
  const dir="/tmp/icons";
  const symbols=readdirSync(dir).filter(f=>f.endsWith(".svg")).sort().map(f=>{
    const inner=readFileSync(dir+"/"+f,"utf8")
      .replace(/<!--[\s\S]*?-->/g,"").replace(/^[\s\S]*?<svg[^>]*>/,"")
      .replace(/<\/svg>\s*$/,"").replace(/\s+/g," ").trim();
    return `<symbol id="i-${f.replace(/\.svg$/,"")}" viewBox="0 0 24 24">${inner}</symbol>`;
  });
  console.log(symbols.length+" icons");
'
```

Then check nothing is unreferenced: `node docs/design/mockups/check.mjs`.

Lucide is ISC licensed. Icons are the icon set §12 fixes for the product, so the deck and the
app draw from the same set by construction.

// Extracts the icon path data from the same Phosphor icons the extension imports, so
// the shapes are the real ones rather than an approximation drawn by hand.
//
//   node apps/macos/testdata/icons/genicons.mjs
//
// The extension renders <AppIcon name="close" size={14} />, which is Phosphor's X at
// weight "regular" in a 256x256 viewBox. Reproducing a close button means reproducing
// that path, not inventing a cross.
import { readFileSync, writeFileSync } from "node:fs";
import { fileURLToPath, pathToFileURL } from "node:url";
import path from "node:path";

const here = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(here, "../../../..");

// The alias-to-component map, read from the extension's own switch.
const iconsSource = readFileSync(path.join(repoRoot, "src/ui/icons.tsx"), "utf8");
const aliases = {};
{
  const re = /case "([a-zA-Z]+)":\s*\n\s*return <([A-Za-z]+)/g;
  let m;
  while ((m = re.exec(iconsSource)) !== null) aliases[m[1]] = m[2];
}
if (Object.keys(aliases).length === 0) throw new Error("no icons could be parsed from src/ui/icons.tsx");

// Every icon's "regular" weight, read from Phosphor's generated definitions.
const icons = {};
for (const [alias, component] of Object.entries(aliases)) {
  const defPath = path.join(repoRoot, "node_modules/@phosphor-icons/react/dist/defs", component + ".es.js");
  const def = await import(pathToFileURL(defPath).href);
  const weights = def.default;
  const regular = weights.get ? weights.get("regular") : weights.regular;
  if (!regular) throw new Error(`${component} has no regular weight`);

  // The element tree is React elements; the path data is on the "d" prop.
  const paths = [];
  const walk = (node) => {
    if (!node || typeof node !== "object") return;
    if (Array.isArray(node)) return node.forEach(walk);
    const props = node.props;
    if (props?.d) paths.push(props.d);
    if (props?.children) walk(props.children);
  };
  walk(regular);
  if (paths.length === 0) throw new Error(`${component} produced no path data`);

  icons[alias] = { component, viewBox: 256, paths };
}

writeFileSync(
  path.join(here, "icons.json"),
  JSON.stringify({ generatedFrom: "src/ui/icons.tsx + @phosphor-icons/react", icons }, null, 2) + "\n"
);
console.log(`wrote icons.json: ${Object.keys(icons).length} icons, ` +
  `${Object.values(icons).reduce((n, i) => n + i.paths.length, 0)} paths`);

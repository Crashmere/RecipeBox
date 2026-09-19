import { execFileSync } from "node:child_process";
import { mkdtempSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import { fileURLToPath } from "node:url";

const publicDir = fileURLToPath(new URL("../public/", import.meta.url));
const temporary = mkdtempSync(path.join(tmpdir(), "recipebox-icons-"));
const source = path.join(publicDir, "recipebox.svg");
try {
  const sizes = [16, 32, 48];
  const images = sizes.map((size) => {
    const target = path.join(temporary, size + ".png");
    execFileSync("vips", [
      "thumbnail",
      source,
      target + "[strip]",
      String(size),
    ]);
    return readFileSync(target);
  });
  // ICO directory followed by PNG frames for each browser icon size.
  const directory = Buffer.alloc(6 + sizes.length * 16);
  directory.writeUInt16LE(1, 2);
  directory.writeUInt16LE(sizes.length, 4);
  let offset = directory.length;
  images.forEach((data, index) => {
    const entry = 6 + index * 16;
    directory[entry] = sizes[index];
    directory[entry + 1] = sizes[index];
    directory.writeUInt16LE(1, entry + 4);
    directory.writeUInt16LE(32, entry + 6);
    directory.writeUInt32LE(data.length, entry + 8);
    directory.writeUInt32LE(offset, entry + 12);
    offset += data.length;
  });
  writeFileSync(
    path.join(publicDir, "favicon.ico"),
    Buffer.concat([directory, ...images]),
  );
  const touch = path.join(temporary, "touch.png");
  execFileSync("vips", ["thumbnail", source, touch, "180"]);
  execFileSync("vips", [
    "flatten",
    touch,
    path.join(publicDir, "apple-touch-icon.png") + "[strip]",
    "--background=184 70 49",
  ]);
  console.log(
    "Generated favicon.ico (16/32/48 px) and apple-touch-icon.png (180 px)",
  );
} finally {
  rmSync(temporary, { recursive: true, force: true });
}

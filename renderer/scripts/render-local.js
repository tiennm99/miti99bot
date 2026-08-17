import {mkdir, writeFile} from 'node:fs/promises';
import path from 'node:path';
import {renderWheelGif} from '../src/render/render-gif.js';
import {parseRenderLocalArgs} from './render-local-args.js';

const usage = `Usage:
  npm run render:local -- --option <text> --option <text> [options]

Options:
  -o, --output <path>   Output file (default: wheel.gif)
      --option <text>   Wheel option; repeat at least twice
      --winner <index>  Zero-based winner index (random when omitted)
      --duration <ms>   Spin duration, 3000-10000 (default: 6500)
      --hold <ms>       Winner hold, 500-2500 (default: 1200)
      --fps <value>     12, 15, or 20 (default: 15)
      --size <pixels>   384, 480, or 512 (default: 512)
      --theme <name>    classic, festival, or mono
      --timeout <ms>    Render timeout, minimum 7000 (default: 30000)
  -h, --help            Show this help`;

try {
  const parsed = parseRenderLocalArgs(process.argv.slice(2));
  if (parsed.help) {
    console.log(usage);
    process.exitCode = 0;
  } else {
    const output = path.resolve(parsed.output);
    await mkdir(path.dirname(output), {recursive: true});
    const result = await renderWheelGif(parsed.request, {
      timeoutInMilliseconds: parsed.timeoutInMilliseconds,
    });
    await writeFile(output, result.buffer);
    console.log(`${output} ${result.byteLength} bytes ${result.durationMs}ms`);
  }
} catch (error) {
  const message = error instanceof Error ? error.message : String(error);
  console.error(`Unable to render GIF: ${message}\n\n${usage}`);
  process.exitCode = 1;
}


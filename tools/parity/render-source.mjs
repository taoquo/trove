#!/usr/bin/env node
// Renders the extension's options page and writes a screenshot plus a box tree.
//
//   node tools/parity/render-source.mjs --state workspace --width 1297 --height 808
//
// Output goes to docs/parity/source-<state>-<width>.png and .json. The JSON is
// the measured geometry of every rendered element: one row per node, with the
// test id, class and rect. It is the reference the native app is compared
// against, and unlike a hand-taken screenshot it is reproducible and quotable.
//
// No dependencies: Node's global WebSocket drives Chrome's DevTools protocol,
// and the static server is ten lines of node:http.

import { createServer } from "node:http"
import { spawn } from "node:child_process"
import { readFile, writeFile, mkdir } from "node:fs/promises"
import { existsSync, statSync } from "node:fs"
import { extname, join, normalize, resolve } from "node:path"
import { createServer as createNetServer } from "node:net"

const ROOT = resolve(import.meta.dirname, "../..")
const CHROME = process.env.CHROME_BIN || "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"

function arg(name, fallback) {
  const index = process.argv.indexOf(`--${name}`)
  return index === -1 ? fallback : process.argv[index + 1]
}

const STATE = arg("state", "workspace")
const WIDTH = Number(arg("width", "1297"))
const HEIGHT = Number(arg("height", "808"))
const OUT_DIR = resolve(ROOT, arg("out", "docs/parity"))

const MIME = {
  ".html": "text/html; charset=utf-8",
  ".js": "text/javascript; charset=utf-8",
  ".mjs": "text/javascript; charset=utf-8",
  ".css": "text/css; charset=utf-8",
  ".json": "application/json; charset=utf-8",
  ".png": "image/png",
  ".woff2": "font/woff2"
}

async function serve() {
  const server = createServer(async (request, response) => {
    const url = new URL(request.url, "http://127.0.0.1")
    let path = normalize(join(ROOT, decodeURIComponent(url.pathname)))
    // BrandLogo resolves `./assets/...` against the document, which lives under
    // /tools/parity/. The bundle's own assets are next to options.js.
    const vendor = join(ROOT, "tools/parity/assets")
    if (path.startsWith(vendor)) {
      path = join(ROOT, "build/chrome-mv3/assets", path.slice(vendor.length + 1))
    }
    if (!path.startsWith(ROOT) || !existsSync(path) || !statSync(path).isFile()) {
      response.writeHead(404).end("not found")
      return
    }
    response.writeHead(200, { "content-type": MIME[extname(path)] || "application/octet-stream" })
    response.end(await readFile(path))
  })
  await new Promise((done) => server.listen(0, "127.0.0.1", done))
  return server
}

function freePort() {
  return new Promise((done) => {
    const server = createNetServer()
    server.listen(0, "127.0.0.1", () => {
      const { port } = server.address()
      server.close(() => done(port))
    })
  })
}

const sleep = (ms) => new Promise((done) => setTimeout(done, ms))

async function main() {
  if (!existsSync(join(ROOT, "build/chrome-mv3/options.js"))) {
    console.error("build/chrome-mv3/options.js is missing; run: npm run build")
    process.exit(1)
  }

  const staticServer = await serve()
  const { port: staticPort } = staticServer.address()
  const debugPort = await freePort()

  const chrome = spawn(
    CHROME,
    [
      "--headless=new",
      "--disable-gpu",
      "--no-sandbox",
      "--hide-scrollbars",
      "--force-device-scale-factor=2",
      `--remote-debugging-port=${debugPort}`,
      `--user-data-dir=/tmp/xbm-parity-${process.pid}`,
      "about:blank"
    ],
    { stdio: ["ignore", "ignore", "ignore"] }
  )

  try {
    let version = null
    for (let attempt = 0; attempt < 80 && !version; attempt += 1) {
      try {
        version = await (await fetch(`http://127.0.0.1:${debugPort}/json/version`)).json()
      } catch {
        await sleep(100)
      }
    }
    if (!version) throw new Error("Chrome did not expose a DevTools endpoint")

    const url = `http://127.0.0.1:${staticPort}/tools/parity/harness.html?state=${STATE}`
    const target = await (await fetch(`http://127.0.0.1:${debugPort}/json/new?${encodeURIComponent(url)}`, { method: "PUT" })).json()

    const socket = new WebSocket(target.webSocketDebuggerUrl)
    let nextId = 0
    const pending = new Map()
    socket.onmessage = (event) => {
      const message = JSON.parse(event.data)
      if (message.id && pending.has(message.id)) {
        pending.get(message.id)(message)
        pending.delete(message.id)
      }
    }
    await new Promise((done) => (socket.onopen = done))
    const send = (method, params = {}) =>
      new Promise((done) => {
        const id = ++nextId
        pending.set(id, done)
        socket.send(JSON.stringify({ id, method, params }))
      })

    await send("Page.enable")
    await send("Emulation.setDeviceMetricsOverride", {
      width: WIDTH,
      height: HEIGHT,
      deviceScaleFactor: 2,
      mobile: false
    })
    await send("Page.navigate", { url })

    let title = ""
    for (let attempt = 0; attempt < 120; attempt += 1) {
      const probe = await send("Runtime.evaluate", { expression: "document.title", returnByValue: true })
      title = probe.result?.result?.value || ""
      if (title.startsWith("DONE")) break
      await sleep(250)
    }
    if (!title.startsWith("DONE")) throw new Error(`harness did not finish (title: ${title || "empty"})`)

    const boxes = await send("Runtime.evaluate", {
      expression: "document.getElementById('__boxes').textContent",
      returnByValue: true
    })
    const shot = await send("Page.captureScreenshot", { format: "png", captureBeyondViewport: false })

    await mkdir(OUT_DIR, { recursive: true })
    const base = join(OUT_DIR, `source-${STATE}-${WIDTH}`)
    await writeFile(`${base}.json`, boxes.result.result.value)
    await writeFile(`${base}.png`, Buffer.from(shot.result.data, "base64"))
    console.log(`${base}.png  ${title}`)

    socket.close()
  } finally {
    chrome.kill()
    staticServer.close()
  }
}

main().catch((error) => {
  console.error(error.message)
  process.exit(1)
})

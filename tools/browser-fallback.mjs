import process from 'node:process'
import { chromium } from 'playwright'

const userUrl = process.argv[2]
const maxItems = Math.max(1, Math.min(500, Number(process.argv[3] || 50)))
const headless = String(process.argv[4] || 'true') !== 'false'

if (!userUrl) {
  console.error('user URL is required')
  process.exit(2)
}

let cookies = {}
try {
  cookies = JSON.parse(process.env.DOUYIN_BROWSER_COOKIES || '{}')
} catch (_) {
  cookies = {}
}

const browser = await chromium.launch({ headless })
try {
  const context = await browser.newContext({
    viewport: { width: 1440, height: 1000 },
    userAgent:
      'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/139.0.0.0 Safari/537.36',
  })

  const cookieEntries = Object.entries(cookies)
    .filter(([, value]) => value !== undefined && value !== null && String(value) !== '')
    .map(([name, value]) => ({
      name,
      value: String(value),
      domain: '.douyin.com',
      path: '/',
      secure: true,
      sameSite: 'Lax',
    }))
  if (cookieEntries.length) await context.addCookies(cookieEntries)

  const page = await context.newPage()
  await page.goto(userUrl, { waitUntil: 'domcontentloaded', timeout: 60000 })
  await page.waitForTimeout(1500)

  const found = new Map()
  let idleRounds = 0
  let previousSize = 0
  for (let round = 0; round < 120 && found.size < maxItems; round++) {
    const links = await page.locator('a[href*="/video/"], a[href*="/note/"], a[href*="/gallery/"]').evaluateAll((nodes) =>
      nodes.map((node) => node.href || node.getAttribute('href') || ''),
    )

    for (const href of links) {
      const match = href.match(/\/(video|note|gallery)\/(\d+)/)
      if (!match) continue
      const [, kind, awemeId] = match
      if (!found.has(awemeId)) {
        found.set(awemeId, {
          aweme_id: awemeId,
          type: kind === 'video' ? 'video' : 'images',
          url: `https://www.douyin.com/video/${awemeId}`,
        })
      }
      if (found.size >= maxItems) break
    }

    if (found.size === previousSize) idleRounds += 1
    else idleRounds = 0
    previousSize = found.size
    if (idleRounds >= 6) break

    await page.evaluate(() => window.scrollTo(0, document.body.scrollHeight))
    await page.waitForTimeout(900)
  }

  process.stdout.write(JSON.stringify({ items: [...found.values()].slice(0, maxItems) }))
} finally {
  await browser.close()
}

import fs from 'node:fs/promises'
import path from 'node:path'
import process from 'node:process'
import { chromium } from 'playwright'

const output = path.resolve(process.argv[2] || '.cookies.json')
// ttwid/odin_tt are set for anonymous visitors too; sessionid only appears
// after a real login, so it is the actual completion signal.
const required = ['ttwid', 'odin_tt', 'passport_csrf_token', 'sessionid']

const browser = await chromium.launch({ headless: false })
try {
  const context = await browser.newContext()
  const page = await context.newPage()
  await page.goto('https://www.douyin.com/', { waitUntil: 'domcontentloaded', timeout: 60000 })
  console.log('请在打开的浏览器中完成抖音登录。检测到关键 Cookie 后会自动保存并退出。')

  const deadline = Date.now() + 10 * 60 * 1000
  while (Date.now() < deadline) {
    const cookies = await context.cookies('https://www.douyin.com/')
    const map = Object.fromEntries(cookies.map((item) => [item.name, item.value]))
    const ready = required.every((key) => map[key])
    if (ready) {
      await fs.mkdir(path.dirname(output), { recursive: true })
      await fs.writeFile(output, JSON.stringify(map, null, 2), { mode: 0o600 })
      console.log(`Cookie 已保存到 ${output}`)
      process.exitCode = 0
      break
    }
    await page.waitForTimeout(1500)
  }

  if (Date.now() >= deadline) {
    console.error('等待登录超时，未写入 Cookie。')
    process.exitCode = 1
  }
} finally {
  await browser.close()
}

"use strict";

/**
 * CLI options:
 *
 * width    - browser viewport width
 * height   - browser viewport height
 * url      - site url address
 * path     - the file path to save the image to
 * fullPage - when true, takes a screenshot of the full scrollable page
 * selector - css element selector
 *
 * ENV:
 *
 * PUPPETEER_EXECUTABLE_PATH=/usr/bin/chromium-browser or path to google-chrome
 *
 * Examples:
 *
 * node screenshot.js --url=https://vc.ru/ --path=./screenshot.png --width=320 --height=1000
 * node screenshot.js --url=https://vc.ru/ --path=./screenshot.png --fullPage
 * node screenshot.js --url=https://vc.ru/ --path=./screenshot.png --selector="div.site-header__item:nth-child(2)"
 *
 */

const puppeteer = require("puppeteer");

const minimal_args = [
  "--autoplay-policy=user-gesture-required",
  "--disable-background-networking",
  "--disable-background-timer-throttling",
  "--disable-backgrounding-occluded-windows",
  "--disable-breakpad",
  "--disable-client-side-phishing-detection",
  "--disable-component-update",
  "--disable-default-apps",
  "--disable-dev-shm-usage",
  "--disable-domain-reliability",
  "--disable-extensions",
  "--disable-features=AudioServiceOutOfProcess",
  "--disable-hang-monitor",
  "--disable-ipc-flooding-protection",
  "--disable-notifications",
  "--disable-offer-store-unmasked-wallet-cards",
  "--disable-popup-blocking",
  "--disable-print-preview",
  "--disable-prompt-on-repost",
  "--disable-renderer-backgrounding",
  "--disable-setuid-sandbox",
  "--disable-speech-api",
  "--disable-sync",
  "--hide-scrollbars",
  "--ignore-gpu-blacklist",
  "--metrics-recording-only",
  "--mute-audio",
  "--no-default-browser-check",
  "--no-first-run",
  "--no-pings",
  "--no-sandbox",
  "--no-zygote",
  "--password-store=basic",
  "--use-gl=swiftshader",
  "--use-mock-keychain",
];

(async function () {
  try {
    const {
      width = 1920,
      height = 800,
      url = "",
      path = "",
      fullPage = false,
      selector = "",
    } = require("minimist")(process.argv.slice(2));
    const browser = await puppeteer.launch({
      headless: true,
      args: minimal_args,
      userDataDir: "./cacheDir",
    //   product: 'firefox',
    });

    const page = await browser.newPage();

    const blocked_domains = ["googlesyndication.com", "adservice.google.com"];

    await page.setRequestInterception(true);
    page.on("request", (request) => {
      const url = request.url();
      if (blocked_domains.some((domain) => url.includes(domain))) {
        request.abort();
      } else if (request.resourceType() === "image") {
        request.abort();
      } else {
        request.continue();
      }
    });

    await page.setCacheEnabled(false);
    await page.setViewport({ width, height });
    await page.goto(url, { waitUntil: "networkidle2" });
    // await page.goto(url, {waitUntil: 'domcontentloaded'});
    await page.goto(url);

    if (selector) {
      await page.waitForSelector(selector);
      await (await page.$(selector)).screenshot({ path });
    } else {
      await page.screenshot({ path, fullPage });
    }

    await page.close();
    await browser.close();
  } catch (e) {
    console.error(e);
    process.exit(1);
  }
})();

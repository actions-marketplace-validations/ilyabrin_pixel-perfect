'use strict';

const puppeteer = require('puppeteer');

/**
 * CLI options:
 *
 * width - browser viewport width
 * height - browser viewport height
 * url - site url address
 * path - the file path to save the image to
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
(async function () {
    try {
        const {
            width = 1920,
            height = 800,
            url = '',
            path = '',
            fullPage = false,
            selector = '',
        } = require('minimist')(process.argv.slice(2));
        const browser = await puppeteer.launch({
            headless: true,
            args: [
                "--no-sandbox",
                "--disable-gpu",
                "--disable-dev-shm-usage",
            ],
        });
        const page = await browser.newPage();

        await page.setCacheEnabled(false);
        await page.setViewport({ width, height });
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

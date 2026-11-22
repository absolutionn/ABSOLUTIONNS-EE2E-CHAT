import assert from "node:assert/strict";
import { chromium } from "playwright";

const BASE = process.env.BASE_URL || "http://127.0.0.1:18080";

const browser = await chromium.launch();
try {
  const ctx1 = await browser.newContext();
  const ctx2 = await browser.newContext();
  const ctx3 = await browser.newContext();
  const p1 = await ctx1.newPage();
  const p2 = await ctx2.newPage();
  const p3 = await ctx3.newPage();

  // 1. Happy path: create room -> join -> E2EE handshake -> bidirectional messages
  await p1.goto(BASE);
  await p1.click("#create-btn");
  await p1.waitForSelector("#room-url:not(:empty)");
  const roomUrl = (await p1.textContent("#room-url")).trim();
  const roomId = new URL(roomUrl).searchParams.get("room");

  await p2.goto(roomUrl);
  await p1.waitForFunction(() => document.querySelector("#status")?.textContent.includes("ключ узгоджено"));
  await p2.waitForFunction(() => document.querySelector("#status")?.textContent.includes("ключ узгоджено"));

  // NFR-1 check: nothing stored in localStorage
  assert.equal(await p1.evaluate(() => localStorage.length), 0);
  assert.equal(await p2.evaluate(() => localStorage.length), 0);

  await p1.fill("#msg-input", "Привіт з E2EE");
  await p1.click("#send-btn");
  await p2.waitForFunction(() => document.querySelector("#messages")?.textContent.includes("Привіт з E2EE"));

  await p2.fill("#msg-input", "Відповідь підтверджено");
  await p2.click("#send-btn");
  await p1.waitForFunction(() => document.querySelector("#messages")?.textContent.includes("Відповідь підтверджено"));

  // 2. Negative path: 3rd user blocked in UI + direct HTTP probe returns 403
  await p3.goto(roomUrl);
  await p3.waitForSelector("#error-box:not([hidden])");

  const directResp = await fetch(`${BASE}/ws?room=${roomId}`);
  assert.equal(directResp.status, 403, `expected direct HTTP 403, got ${directResp.status}`);

  // Verify existing 2-peer session still works after 3rd-party attempt
  await p1.fill("#msg-input", "Після блокування 3-го");
  await p1.click("#send-btn");
  await p2.waitForFunction(() => document.querySelector("#messages")?.textContent.includes("Після блокування 3-го"));

  console.log("E2E PASSED: happy path, NFR-1 localStorage=0, and 3rd-user 403 block verified.");
} finally { await browser.close() };

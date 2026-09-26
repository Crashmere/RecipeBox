import { test, expect, type Page } from "@playwright/test";
async function fits(page: Page) {
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
  ).toBe(true);
  const bad = await page
    .locator("input:not([type=hidden]),textarea,select")
    .evaluateAll((els) =>
      els
        .filter((el) => {
          const r = el.getBoundingClientRect();
          return r.width && (r.left < -1 || r.right > innerWidth + 1);
        })
        .map((el) => (el as HTMLInputElement).outerHTML),
    );
  expect(bad).toEqual([]);
}
for (const width of [320, 375, 1440])
  test("recipe and pantry flows at " + width, async ({ page }) => {
    await page.setViewportSize({ width, height: width === 1440 ? 900 : 667 });
    const errors: string[] = [];
    page.on("pageerror", (e) => errors.push(e.message));
    const name = "番茄炒蛋 " + width + " " + Date.now();
    await page.goto("new");
    await page.getByLabel("菜名").fill(name);
    await page.getByLabel("食材 1", { exact: true }).fill("番茄");
    await page.getByLabel("用量 1", { exact: true }).fill("2 个");
    await page.getByRole("button", { name: "添加食材", exact: true }).click();
    await page.getByLabel("食材 2", { exact: true }).fill("鸡蛋");
    await page.getByLabel("用量 2", { exact: true }).fill("3 个");
    await page
      .getByLabel("步骤 1", { exact: true })
      .fill("切好番茄，小火炒蛋。");
    await page.getByRole("button", { name: "添加步骤", exact: true }).click();
    await page
      .getByLabel("步骤 2", { exact: true })
      .fill("一起翻炒，最后加盐。");
    await page.getByLabel("上移步骤 2", { exact: true }).click();
    await expect(page.getByLabel("步骤 1", { exact: true })).toHaveValue(
      "一起翻炒，最后加盐。",
    );
    await fits(page);
    await page.getByRole("button", { name: "保存菜谱", exact: true }).click();
    await expect(
      page.getByRole("heading", { name, exact: true }),
    ).toBeVisible();
    const detail = page.url();
    await page.getByRole("button", { name: "记一次下厨", exact: true }).click();
    await page.getByLabel("下厨心得").fill("再少放一点盐。");
    await page.getByRole("button", { name: "5 星", exact: true }).click();
    await fits(page);
    await page.getByRole("button", { name: "保存这次下厨" }).click();
    await expect(
      page.getByText("再少放一点盐。", { exact: true }),
    ).toBeVisible();
    await page.getByRole("button", { name: "修改", exact: true }).click();
    await page.getByLabel("下厨心得").fill("味道很好，继续保持。");
    await page.getByRole("button", { name: "保存这次下厨" }).click();
    await expect(
      page.getByText("味道很好，继续保持。", { exact: true }),
    ).toBeVisible();
    await page.getByRole("button", { name: "大字做菜模式" }).click();
    await expect(page.getByRole("dialog")).toBeVisible();
    await fits(page);
    await page.getByRole("button", { name: "下一步" }).click();
    await page.getByRole("button", { name: "关闭", exact: true }).click();
    await page.getByRole("link", { name: "编辑菜谱" }).click();
    await page.getByRole("button", { name: "添加步骤", exact: true }).click();
    await page.getByLabel("步骤 3", { exact: true }).fill("出锅前撒一把葱花。");
    await page.reload();
    await expect(page.getByLabel("步骤 3", { exact: true })).toHaveValue(
      "出锅前撒一把葱花。",
    );
    await expect(page.getByText("已恢复上次未保存的草稿。")).toBeVisible();
    await page.getByRole("button", { name: "保存菜谱", exact: true }).click();
    await expect(
      page.getByText("出锅前撒一把葱花。", { exact: true }),
    ).toBeVisible();
    await fits(page);
    await page.getByRole("button", { name: "移入回收站", exact: true }).click();
    await page
      .getByRole("dialog")
      .getByRole("button", { name: "移入回收站", exact: true })
      .click();
    await expect(page).toHaveURL(/\/recipebox\/$/);
    await page.goto("trash");
    const trash = page.locator(".trash-item").filter({ hasText: name });
    await trash.getByRole("button", { name: "恢复" }).click();
    await expect(trash).toHaveCount(0);
    await page.goto(detail);
    await expect(
      page.getByRole("heading", { name, exact: true }),
    ).toBeVisible();
    await page.goto("pantry/new");
    await page.getByLabel("食品名称").fill("挂面 " + name);
    await page.getByLabel("剩余数量").fill("2");
    await page.getByLabel("单位", { exact: true }).fill("包");
    await page.getByLabel("放在哪里").fill("厨房上层橱柜");
    await page.getByLabel("购买日期", { exact: true }).fill("2026-09-01");
    await page.getByLabel("到期日期", { exact: true }).fill("2026-12-01");
    await fits(page);
    await page.getByRole("button", { name: "保存食品", exact: true }).click();
    const item = page
      .locator(".pantry-item")
      .filter({ hasText: "挂面 " + name });
    await expect(item).toBeVisible();
    await item.getByRole("button", { name: "吃掉一包" }).click();
    await expect(item.locator(".pantry-quantity strong")).toHaveText("1");
    await item.getByRole("button", { name: "吃掉一包" }).click();
    await expect(item).toHaveCount(0);
    await page.getByRole("button", { name: "已经吃完", exact: true }).click();
    await expect(item).toBeVisible();
    await fits(page);
    expect(errors).toEqual([]);
  });
test("upload photo, set cover, preview and persist", async ({ page }) => {
  await page.goto("new");
  const name = "有照片的菜 " + Date.now();
  await page.getByLabel("菜名").fill(name);
  await page
    .locator("input[type=file][multiple]")
    .setInputFiles("public/apple-touch-icon.png");
  await expect(page.locator(".photo-grid .photo-tile")).toHaveCount(1);
  await expect(
    page.getByRole("button", { name: "保存菜谱", exact: true }),
  ).toBeEnabled();
  await page.getByRole("button", { name: "预览照片", exact: true }).click();
  await expect(page.getByRole("dialog").locator("img")).toBeVisible();
  await page.getByRole("button", { name: "关闭", exact: true }).click();
  await page.getByRole("button", { name: "保存菜谱", exact: true }).click();
  await expect(page.getByRole("heading", { name, exact: true })).toBeVisible();
  await page.reload();
  await expect(page.locator(".detail-cover>button>img")).toBeVisible();
  expect(
    await page
      .locator(".detail-cover>button>img")
      .evaluate((el: HTMLImageElement) => el.naturalWidth),
  ).toBeGreaterThan(0);
});
test("recipe photos sit in basic info and keep an existing story", async ({
  page,
  request,
}) => {
  const name = "保留小故事 " + Date.now();
  const created = await (
    await request.post("api/records", {
      headers: { "Idempotency-Key": crypto.randomUUID().replace(/-/g, "") },
      data: { kind: "recipe", name, notes: "奶奶的拿手菜" },
    })
  ).json();
  await page.goto("edit/" + created.id);
  const basics = page.locator(".panel", {
    has: page.getByRole("heading", { name: "基本信息" }),
  });
  await expect(basics.getByRole("group", { name: "美味留影" })).toBeVisible();
  await expect(page.getByLabel("这道菜的小故事")).toHaveCount(0);
  await page
    .locator("input[type=file][multiple]")
    .setInputFiles("public/apple-touch-icon.png");
  await expect(basics.locator(".photo-grid .photo-tile")).toHaveCount(1);
  await fits(page);
  await page.getByRole("button", { name: "保存菜谱", exact: true }).click();
  await expect(page.getByRole("heading", { name, exact: true })).toBeVisible();
  await expect(page.locator(".detail-cover>button>img")).toBeVisible();
  const saved = await (await request.get("api/records/" + created.id)).json();
  expect(saved.notes).toBe("奶奶的拿手菜");
  expect(saved.photoIds).toHaveLength(1);
});
test("lost response recovers on reload without duplicate recipe", async ({
  page,
  request,
}) => {
  const name = "保存响应丢失 " + Date.now();
  await page.goto("new");
  await page.getByLabel("菜名").fill(name);
  let dropped = false;
  await page.route("**/api/records", async (route) => {
    if (route.request().method() === "POST" && !dropped) {
      dropped = true;
      await route.fetch();
      await route.abort("failed");
    } else await route.continue();
  });
  await page.getByRole("button", { name: "保存菜谱", exact: true }).click();
  await expect(
    page.getByRole("button", { name: "核对上次保存", exact: true }),
  ).toBeVisible();
  await page.reload();
  await expect(page.getByRole("heading", { name, exact: true })).toBeVisible();
  const items = await (await request.get("api/records")).json();
  expect(items.filter((e: any) => e.name === name)).toHaveLength(1);
});
test("stalled cook log response finishes from the committed result", async ({
  page,
  request,
}) => {
  const name = "日记响应卡住 " + Date.now();
  const created = await (
    await request.post("api/records", {
      headers: { "Idempotency-Key": crypto.randomUUID().replace(/-/g, "") },
      data: { kind: "recipe", name },
    })
  ).json();
  await page.goto("recipes/" + created.id);
  await page.route("**/api/records/" + created.id, async (route) => {
    if (route.request().method() === "POST") await route.fetch();
    else await route.continue();
  });
  await page.getByRole("button", { name: "记一次下厨", exact: true }).click();
  await page.getByLabel("下厨心得").fill("响应丢了也能记下。");
  await page.getByRole("button", { name: "保存这次下厨" }).click();
  await expect(
    page.getByText("响应丢了也能记下。", { exact: true }),
  ).toBeVisible({ timeout: 10000 });
  await expect(page.getByRole("dialog")).toHaveCount(0);
  const saved = await (await request.get("api/records/" + created.id)).json();
  expect(saved.logs).toHaveLength(1);
});
test("conflict keeps input and can compare latest content", async ({
  page,
  request,
}) => {
  const name = "冲突核对 " + Date.now();
  const key = () => Math.random().toString(16).slice(2).padEnd(32, "0");
  const initial = await (
    await request.post("api/records", {
      headers: { "Idempotency-Key": key() },
      data: { kind: "recipe", name },
    })
  ).json();
  await page.goto("edit/" + initial.id);
  await page.getByLabel("步骤 1", { exact: true }).fill("这台设备的草稿");
  await request.post("api/records/" + initial.id, {
    headers: { "Idempotency-Key": key() },
    data: { ...initial, steps: ["另一台设备的新内容"] },
  });
  await page.getByRole("button", { name: "保存菜谱", exact: true }).click();
  await expect(page.getByRole("alert")).toContainText("另一台设备更新");
  await expect(page.getByLabel("步骤 1", { exact: true })).toHaveValue(
    "这台设备的草稿",
  );
  await page.getByRole("button", { name: "对照草稿与最新版本" }).click();
  await expect(page.getByRole("dialog")).toContainText("这台设备的草稿");
  await page.getByRole("button", { name: "保留此草稿，继续合并" }).click();
  await page.getByLabel("步骤 1", { exact: true }).fill("合并两台设备的心得");
  await page.getByRole("button", { name: "保存菜谱", exact: true }).click();
  await expect(
    page.getByText("合并两台设备的心得", { exact: true }),
  ).toBeVisible();
});
test("failed image upload exposes retry and prevents premature save", async ({
  page,
}) => {
  await page.goto("new");
  await page.getByLabel("菜名").fill("重试照片 " + Date.now());
  let fail = true;
  await page.route("**/api/uploads", async (route) => {
    if (fail) {
      fail = false;
      await route.abort("failed");
    } else await route.continue();
  });
  await page
    .locator("input[type=file][multiple]")
    .setInputFiles("public/apple-touch-icon.png");
  await expect(
    page.getByRole("button", { name: "重试", exact: true }),
  ).toBeVisible();
  await expect(
    page.getByRole("button", { name: "保存菜谱", exact: true }),
  ).toBeDisabled();
  await page.getByRole("button", { name: "重试", exact: true }).click();
  await expect(
    page.getByRole("button", { name: "保存菜谱", exact: true }),
  ).toBeEnabled();
  await expect(
    page.getByRole("button", { name: "预览照片", exact: true }),
  ).toBeVisible();
});

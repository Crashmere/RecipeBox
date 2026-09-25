import { describe, it, expect, vi, afterEach } from "vitest";
import { write } from "./api";
afterEach(() => {
  vi.useRealTimers();
  vi.unstubAllGlobals();
});
const saved = { id: "a".repeat(32), revision: 2 };
const json = (status: number, body: unknown) =>
  Promise.resolve(new Response(JSON.stringify(body), { status }));
describe("write", () => {
  it("returns the save response without looking up the operation", async () => {
    const fetch = vi.fn(() => json(200, saved));
    vi.stubGlobal("fetch", fetch);
    await expect(write(saved.id, { revision: 1 })).resolves.toEqual(saved);
    expect(fetch).toHaveBeenCalledTimes(1);
  });
  it("finishes from the committed operation when the response is lost", async () => {
    vi.useFakeTimers();
    let key = "";
    let lookups = 0;
    const fetch = vi.fn((url: string, init?: RequestInit) => {
      if (init?.method === "POST") {
        key = (init.headers as Record<string, string>)["Idempotency-Key"];
        return new Promise<Response>(() => {});
      }
      expect(url).toMatch(new RegExp("api/operations/" + key + "$"));
      return ++lookups === 1
        ? json(404, { message: "提交记录不存在" })
        : json(200, saved);
    });
    vi.stubGlobal("fetch", fetch);
    const result = write(saved.id, { revision: 1, action: "log" });
    await vi.advanceTimersByTimeAsync(3000);
    expect(lookups).toBe(1);
    await vi.advanceTimersByTimeAsync(3000);
    await expect(result).resolves.toEqual(saved);
    expect(
      fetch.mock.calls.filter(([, i]) => i?.method === "POST"),
    ).toHaveLength(1);
  });
});

import { ref } from "vue";
import type { Entry } from "./types";
export const base = import.meta.env.BASE_URL;
export const media = (id: string, thumb = false) =>
  base + "api/media/" + id + "/" + (thumb ? "thumb" : "main");
export const uid = () =>
  Array.from(crypto.getRandomValues(new Uint8Array(16)), (x) =>
    x.toString(16).padStart(2, "0"),
  ).join("");
export class ApiError extends Error {
  constructor(
    message: string,
    public status: number,
  ) {
    super(message);
  }
}
export async function api<T>(
  path: string,
  init?: RequestInit,
  timeout = 120000,
): Promise<T> {
  let r: Response;
  try {
    r = await fetch(base + "api/" + path, {
      ...init,
      signal: AbortSignal.timeout(timeout),
    });
  } catch {
    throw new ApiError("网络连接中断，输入已保留。请检查网络后重试。", 0);
  }
  if (!r.ok) {
    let data;
    try {
      data = await r.json();
    } catch {}
    throw new ApiError(data?.message || "请求失败，请稍后再试", r.status);
  }
  return r.json();
}
export function stored<T>(key: string): T | null {
  try {
    return JSON.parse(sessionStorage.getItem("recipebox:" + key) || "null");
  } catch {
    return null;
  }
}
export function persist(key: string, value: unknown) {
  try {
    sessionStorage.setItem("recipebox:" + key, JSON.stringify(value));
  } catch {
    notify("浏览器无法保存草稿，请保持此页面打开");
  }
}
export function forget(key: string) {
  try {
    sessionStorage.removeItem("recipebox:" + key);
  } catch {}
}
export async function write(id: string, data: unknown): Promise<Entry> {
  const path = "records" + (id ? "/" + id : "");
  const body = JSON.stringify(data);
  const storage = "pending:" + path;
  const previous = stored<{ body: string; key: string }>(storage);
  if (previous && previous.body !== body) {
    let recovered: Entry | null = null;
    try {
      recovered = await api<Entry>("operations/" + previous.key);
    } catch (e) {
      if (!(e instanceof ApiError && e.status === 404)) throw e;
    }
    if (recovered) {
      forget(storage);
      throw new ApiError(
        "上次操作已成功。请刷新页面查看最新结果，再继续修改。",
        409,
      );
    }
    throw new ApiError("上次操作结果待确认，请刷新页面核对后重试原操作。", 409);
  }
  const key = previous?.body === body ? previous.key : uid();
  persist(storage, { body, key });
  let done = false;
  try {
    const result = await Promise.race([
      api<Entry>(path, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "Idempotency-Key": key,
        },
        body,
      }),
      committed(key, () => done),
    ]);
    forget(storage);
    return result;
  } catch (e) {
    if (e instanceof ApiError && e.status >= 400 && e.status < 500) {
      forget(storage);
    }
    throw e;
  } finally {
    done = true;
  }
}
const sleep = (ms: number) => new Promise((r) => setTimeout(r, ms));
// The network can lose a response after the server has committed; the
// operation lookup ends the wait without resubmitting.
async function committed(key: string, done: () => boolean): Promise<Entry> {
  for (;;) {
    await sleep(3000);
    if (done()) return new Promise<Entry>(() => {});
    try {
      return await api<Entry>("operations/" + key, undefined, 5000);
    } catch {}
  }
}
export const message = ref("");
let timer: ReturnType<typeof setTimeout>;
export function notify(text: string) {
  message.value = text;
  clearTimeout(timer);
  timer = setTimeout(() => (message.value = ""), 4500);
}

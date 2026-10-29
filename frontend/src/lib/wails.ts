import { Events, Browser, Clipboard } from "@wailsio/runtime";

/**
 * Registers an event listener that unwraps the Wails v3 event data.
 * Returns an unsubscribe function.
 */
export function onEvent<T = unknown>(name: string, handler: (data: T) => void): () => void {
  return Events.On(name, (ev: unknown) => {
    if (ev && typeof ev === "object" && "data" in ev) {
      handler((ev as { data: T }).data);
    } else {
      handler(ev as T);
    }
  });
}

/**
 * Opens a URL in the default system browser.
 */
export function openBrowser(url: string): void {
  Browser.OpenURL(url);
}

/**
 * Copies text to system clipboard.
 */
export async function copyToClipboard(text: string): Promise<boolean> {
  try {
    await Clipboard.SetText(text);
    return true;
  } catch {
    return false;
  }
}

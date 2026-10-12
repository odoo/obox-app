/** Centralized localStorage key registry (KEYS) and typed get/set/remove helpers plus a usePref() React hook for reactive UI preferences.
 *
 * Usage:
 *   import { ls, KEYS } from "@/utils/localStorageHelper";
 *
 *   ls.set(KEYS.SIDEBAR_COLLAPSED, true);
 *   const collapsed = ls.get(KEYS.SIDEBAR_COLLAPSED, false);
 */

import { useState, useCallback } from "react";

// ── Key registry ─────────────────────────────────────────────────────────────
// Add new keys here — never use raw strings elsewhere in the codebase.

export const KEYS = {
    /** PIN security & rate-limiting */
    PIN_ATTEMPTS: "epos-pin-attempts",
    PIN_COOLDOWN_UNTIL: "epos-pin-cooldown-until",
} as const;

// Derive a union type of all keys for type safety
export type AppStorageKey = (typeof KEYS)[keyof typeof KEYS];

/**
 * Keys that survive a daily cache clear.
 * Import this in cacheManager.ts instead of duplicating the list.
 */
export const PRESERVED_KEYS: AppStorageKey[] = [];

// ── Core helpers ──────────────────────────────────────────────────────────────

function safeGet<T>(key: string, fallback: T): T {
    try {
        const raw = localStorage.getItem(key);
        if (raw === null) return fallback;
        return JSON.parse(raw) as T;
    } catch {
        return fallback;
    }
}

function safeSet<T>(key: string, value: T): void {
    try {
        localStorage.setItem(key, JSON.stringify(value));
    } catch (e) {
        console.warn(`[localStorage] Failed to write key "${key}"`, e);
    }
}

function safeRemove(key: string): void {
    try {
        localStorage.removeItem(key);
    } catch (e) {
        console.warn(`[localStorage] Failed to remove key "${key}"`, e);
    }
}

/**
 * Primary helper object. All reads and writes go through here.
 *
 * @example
 * ls.set(KEYS.SIDEBAR_COLLAPSED, true);
 * ls.get(KEYS.SIDEBAR_COLLAPSED, false);
 * ls.remove(KEYS.SIDEBAR_COLLAPSED);
 */
export const ls = {
    /** Read a value, returning `fallback` if absent or unparsable. */
    get<T>(key: AppStorageKey, fallback: T): T {
        return safeGet(key, fallback);
    },

    /** Write a value (JSON-serialised). */
    set<T>(key: AppStorageKey, value: T): void {
        safeSet(key, value);
    },

    /** Remove a key. */
    remove(key: AppStorageKey): void {
        safeRemove(key);
    },

    /**
     * Clear everything except `PRESERVED_KEYS`.
     * Mirrors the logic in cacheManager.ts — prefer calling this from there.
     */
    clearNonPreserved(): void {
        const saved: Record<string, string | null> = {};
        PRESERVED_KEYS.forEach((k) => {
            saved[k] = localStorage.getItem(k);
        });

        localStorage.clear();

        Object.entries(saved).forEach(([k, v]) => {
            if (v !== null) localStorage.setItem(k, v);
        });
    },
};

// ── React hook ────────────────────────────────────────────────────────────────

/**
 * `usePref` — a `useState`-like hook that is backed by localStorage.
 *
 * The value is read once on mount and written back on every update.
 * Changes in other tabs are NOT synced (add a `storage` event listener
 * yourself if cross-tab reactivity is needed).
 *
 * @example
 * const [collapsed, setCollapsed] = usePref(KEYS.SIDEBAR_COLLAPSED, false);
 */
export function usePref<T>(
    key: AppStorageKey,
    defaultValue: T,
): [T, (next: T | ((prev: T) => T)) => void] {
    const [value, setValueRaw] = useState<T>(() => safeGet(key, defaultValue));

    const setValue = useCallback(
        (next: T | ((prev: T) => T)) => {
            setValueRaw((prev) => {
                const resolved = typeof next === "function" ? (next as (p: T) => T)(prev) : next;
                safeSet(key, resolved);
                return resolved;
            });
        },
        [key],
    );

    return [value, setValue];
}



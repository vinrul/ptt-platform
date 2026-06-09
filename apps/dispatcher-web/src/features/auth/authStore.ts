import type { AuthSession } from "../../lib/api";
import { create } from "zustand";

const storageKey = "ptt-fleet.dispatcher-session";

interface AuthState {
  session: AuthSession | null;
  setSession: (session: AuthSession) => void;
  clearSession: () => void;
}

export const useAuthStore = create<AuthState>((set) => ({
  session: readStoredSession(),
  setSession: (session) => {
    window.localStorage.setItem(storageKey, JSON.stringify(session));
    set({ session });
  },
  clearSession: () => {
    window.localStorage.removeItem(storageKey);
    set({ session: null });
  },
}));

function readStoredSession(): AuthSession | null {
  try {
    const value = window.localStorage.getItem(storageKey);
    return value ? (JSON.parse(value) as AuthSession) : null;
  } catch {
    window.localStorage.removeItem(storageKey);
    return null;
  }
}

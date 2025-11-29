import { initializeApp } from "firebase/app";
import { getStorage, connectStorageEmulator } from "firebase/storage";

async function fetchConfig() {
  const res = await fetch("/__/firebase/init.json");
  if (!res.ok) {
    throw new Error(`Failed to fetch Firebase config: ${res.statusText}`);
  }
  return res.json();
}

async function initApp() {
  const config = await fetchConfig();
  // Initialize Firebase app if not already initialized
  return initializeApp(config);
}

// @ts-expect-error: add to window
globalThis.firebaseAppPromise = initApp();

export async function ensureInitialized() {
  // @ts-expect-error: add to window
  return globalThis.firebaseAppPromise;
}

const app = await initApp(); // Use existing app initialized by firebase/init.js
export const storage = getStorage(app, import.meta.env.VITE_GC_STORAGE_BUCKET);

if (!import.meta.env.PROD) {
  connectStorageEmulator(storage, "localhost", 9199);
}

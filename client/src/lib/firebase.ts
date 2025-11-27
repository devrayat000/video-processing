import { initializeApp } from "firebase/app";
import { getStorage } from "firebase/storage";

// Your web app's Firebase configuration
// For Firebase JS SDK v7.20.0 and later, measurementId is optional
const firebaseConfig = {
  apiKey: "AIzaSyDzSddnJju6xCeTuwGCSz5i40l_R_P5btQ",
  authDomain: "shosta-ai.firebaseapp.com",
  projectId: "shosta-ai",
  storageBucket: "shosta-ai.firebasestorage.app",
  messagingSenderId: "319016052566",
  appId: "1:319016052566:web:2de3964ea27836d8d818f7",
  measurementId: "G-1ET91DCSBF",
};

export const app = initializeApp(firebaseConfig);
export const storage = getStorage(app, "gs://video_store_6969f");

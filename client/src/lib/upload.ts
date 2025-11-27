import { ref, uploadBytesResumable, getDownloadURL } from "firebase/storage";
import { storage } from "./firebase";

export async function uploadFileToS3(
  file: File,
  key: string,
  onProgress?: (progress: number) => void
) {
  const objectRef = ref(storage, key);
  const uploadTask = uploadBytesResumable(objectRef, file, {
    customMetadata: {
      "x-original": file.name,
    },
  });
  uploadTask.on("state_changed", (snapshot) => {
    onProgress?.((snapshot.bytesTransferred / snapshot.totalBytes) * 100);
  });
  await uploadTask;

  return getDownloadURL(objectRef);
}

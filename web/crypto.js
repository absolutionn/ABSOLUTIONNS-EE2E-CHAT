const subtle = globalThis.crypto.subtle;
const enc = new TextEncoder();
const dec = new TextDecoder();

export function toB64(buf) {
  const bytes = new Uint8Array(buf);
  let bin = "";
  for (let i = 0; i < bytes.length; i++) bin += String.fromCharCode(bytes[i]);
  return btoa(bin);
}

export function fromB64(str) { return Uint8Array.from(atob(str), (c) => c.charCodeAt(0)); }

export async function generateKeyPair() {
  const kp = await subtle.generateKey({ name: "X25519" }, false, ["deriveBits"]);
  const pubRaw = await subtle.exportKey("raw", kp.publicKey);
  return { privateKey: kp.privateKey, publicKeyB64: toB64(pubRaw) };
}

export async function deriveSharedKey(privateKey, peerPubB64) {
  const peerKey = await subtle.importKey(
    "raw", fromB64(peerPubB64), { name: "X25519" }, false, []
  );
  const bits = await subtle.deriveBits({ name: "X25519", public: peerKey }, privateKey, 256);
  const hkdfKey = await subtle.importKey("raw", bits, "HKDF", false, ["deriveKey"]);
  return subtle.deriveKey(
    { name: "HKDF", hash: "SHA-256", salt: new Uint8Array(32), info: enc.encode("e2ee-chat-v1") },
    hkdfKey,
    { name: "AES-GCM", length: 256 },
    false,
    ["encrypt", "decrypt"]
  );
}

export async function computeFingerprint(pubB64) {
  const hash = new Uint8Array(await subtle.digest("SHA-256", fromB64(pubB64)));
  return Array.from(hash.slice(0, 8))
    .map((b) => b.toString(16).padStart(2, "0").toUpperCase())
    .join(":");
}

export async function encryptMessage(aesKey, plaintext) {
  const iv = globalThis.crypto.getRandomValues(new Uint8Array(12));
  const ct = await subtle.encrypt({ name: "AES-GCM", iv }, aesKey, enc.encode(plaintext));
  return { iv: toB64(iv), ct: toB64(ct) };
}

export async function decryptMessage(aesKey, { iv, ct }) {
  const plainBuf = await subtle.decrypt(
    { name: "AES-GCM", iv: fromB64(iv) }, aesKey, fromB64(ct)
  );
  return dec.decode(plainBuf);
}

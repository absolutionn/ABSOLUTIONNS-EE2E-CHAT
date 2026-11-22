import test from "node:test";
import assert from "node:assert/strict";
import {
  generateKeyPair,
  deriveSharedKey,
  computeFingerprint,
  encryptMessage,
  decryptMessage,
  fromB64,
  toB64,
} from "./crypto.js";

test("FR-2 & NFR-1: X25519 key pair is non-extractable and derives symmetric AES key", async () => {
  const alice = await generateKeyPair();
  const bob = await generateKeyPair();
  assert.equal(alice.privateKey.extractable, false);
  assert.equal(bob.privateKey.extractable, false);

  const keyA = await deriveSharedKey(alice.privateKey, bob.publicKeyB64);
  const keyB = await deriveSharedKey(bob.privateKey, alice.publicKeyB64);
  assert.equal(keyA.extractable, false);

  const enc = await encryptMessage(keyA, "Перевірка E2EE 🔐");
  assert.equal(await decryptMessage(keyB, enc), "Перевірка E2EE 🔐");
});

test("FR-4 boundary: empty string and Unicode round-trip", async () => {
  const a = await generateKeyPair();
  const b = await generateKeyPair();
  const keyA = await deriveSharedKey(a.privateKey, b.publicKeyB64);
  const keyB = await deriveSharedKey(b.privateKey, a.publicKeyB64);

  for (const sample of ["", "a", "Привіт, ВНТУ! ".repeat(50)]) {
    const enc = await encryptMessage(keyA, sample);
    assert.equal(await decryptMessage(keyB, enc), sample);
  }
});

test("STRIDE Tampering: modified ciphertext or wrong key fails AES-GCM verification", async () => {
  const a = await generateKeyPair();
  const b = await generateKeyPair();
  const eve = await generateKeyPair();

  const keyA = await deriveSharedKey(a.privateKey, b.publicKeyB64);
  const keyB = await deriveSharedKey(b.privateKey, a.publicKeyB64);
  const keyEve = await deriveSharedKey(eve.privateKey, a.publicKeyB64);

  const enc = await encryptMessage(keyA, "secret payload");
  const tamperedBytes = fromB64(enc.ct);
  tamperedBytes[0] ^= 0xff;

  await assert.rejects(() => decryptMessage(keyB, { iv: enc.iv, ct: toB64(tamperedBytes) }));
  await assert.rejects(() => decryptMessage(keyEve, enc));
});

test("STRIDE Spoofing: fingerprint is deterministic and unique per public key", async () => {
  const a = await generateKeyPair();
  const b = await generateKeyPair();
  const fp1 = await computeFingerprint(a.publicKeyB64);
  const fp2 = await computeFingerprint(a.publicKeyB64);
  const fpB = await computeFingerprint(b.publicKeyB64);

  assert.equal(fp1, fp2);
  assert.notEqual(fp1, fpB);
  assert.match(fp1, /^([0-9A-F]{2}:){7}[0-9A-F]{2}$/);
});

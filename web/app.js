import {
  generateKeyPair,
  deriveSharedKey,
  computeFingerprint,
  encryptMessage,
  decryptMessage,
} from "./crypto.js";

const lobby = document.getElementById("lobby");
const chat = document.getElementById("chat");
const createBtn = document.getElementById("create-btn");
const roomUrlEl = document.getElementById("room-url");
const statusEl = document.getElementById("status");
const fpEl = document.getElementById("fingerprint");
const errorBox = document.getElementById("error-box");
const messagesEl = document.getElementById("messages");
const msgForm = document.getElementById("msg-form");
const msgInput = document.getElementById("msg-input");
const sendBtn = document.getElementById("send-btn");

let ws = null;
let keyPair = null;
let sharedKey = null;
let pubKeySent = false;
const seenIVs = new Set();

function showError(msg) {
  errorBox.textContent = msg;
  errorBox.hidden = false;
}

function appendMsg(author, text) {
  const li = document.createElement("li");
  li.textContent = `${author}: ${text}`;
  messagesEl.appendChild(li);
}

async function connectRoom(roomId) {
  lobby.hidden = true;
  chat.hidden = false;
  roomUrlEl.textContent = `${location.origin}/?room=${roomId}`;
  keyPair = await generateKeyPair();

  const proto = location.protocol === "https:" ? "wss:" : "ws:";
  let opened = false;
  ws = new WebSocket(`${proto}//${location.host}/ws?room=${encodeURIComponent(roomId)}`);

  ws.onopen = () => { opened = true };

  ws.onmessage = async (ev) => {
    const msg = JSON.parse(ev.data);
    if (msg.type === "status") {
      if (msg.peers === 1) {
        statusEl.textContent = "Очікування учасника";
      } else if (msg.peers === 2) {
        statusEl.textContent = "У кімнаті — обмін ключами";
        if (!pubKeySent) {
          pubKeySent = true;
          ws.send(JSON.stringify({ type: "pubkey", data: keyPair.publicKeyB64 }));
        }
      }
      return;
    }
    if (msg.type === "pubkey") {
      sharedKey = await deriveSharedKey(keyPair.privateKey, msg.data);
      fpEl.textContent = await computeFingerprint(msg.data);
      statusEl.textContent = "У кімнаті — ключ узгоджено";
      msgInput.disabled = false;
      sendBtn.disabled = false;
      return;
    }
    if (msg.type === "msg") {
      if (!sharedKey || seenIVs.has(msg.id)) {
        showError("Відхилено повторне або невалідне повідомлення");
        return;
      }
      seenIVs.add(msg.id);
      try {
        const text = await decryptMessage(sharedKey, msg.data);
        appendMsg("Співрозмовник", text);
      } catch { showError("Помилка перевірки цілісності AES-GCM") };
      return;
    }
    if (msg.type === "error") { showError(`Помилка сервера: ${msg.error}`) };
  };

  ws.onclose = () => {
    if (!opened) {
      statusEl.textContent = "Відхилено";
      showError("Підключення відхилено (кімната заповнена або не існує)");
    }
  };
}

createBtn.addEventListener("click", async () => {
  const res = await fetch("/api/rooms", { method: "POST" });
  const { roomId } = await res.json();
  history.replaceState(null, "", `/?room=${roomId}`);
  await connectRoom(roomId);
});

msgForm.addEventListener("submit", async (e) => {
  e.preventDefault();
  if (!sharedKey || !msgInput.value) return;
  const plain = msgInput.value;
  const encData = await encryptMessage(sharedKey, plain);
  ws.send(JSON.stringify({ type: "msg", id: encData.iv, data: encData }));
  appendMsg("Ви", plain);
  msgInput.value = "";
});

const initialRoom = new URLSearchParams(location.search).get("room");
if (initialRoom) { connectRoom(initialRoom) };

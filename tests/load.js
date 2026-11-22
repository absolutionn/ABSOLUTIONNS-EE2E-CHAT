import http from "k6/http";
import ws from "k6/ws";
import { check } from "k6";

const BASE_HTTP = __ENV.BASE_URL || "http://127.0.0.1:18080";
const BASE_WS = BASE_HTTP.replace(/^http/, "ws");
const isSmoke = __ENV.MODE === "smoke";

export const options = {
  vus: isSmoke ? 2 : 20,
  duration: isSmoke ? "5s" : "15s",
  thresholds: {
    http_req_duration: ["p(95)<50"],
    http_req_failed: ["rate<0.01"],
    checks: ["rate>0.99"],
  },
};

export default function () {
  const res = http.post(`${BASE_HTTP}/api/rooms`, null);
  const okCreate = check(res, {
    "create room status 201": (r) => r.status === 201,
    "roomId returned": (r) => Boolean(r.json("roomId")),
  });
  if (!okCreate) return;

  const roomId = res.json("roomId");
  let gotStatus = false;

  const wsRes = ws.connect(`${BASE_WS}/ws?room=${roomId}`, {}, function (socket) {
    socket.on("message", (raw) => {
      const msg = JSON.parse(raw);
      if (msg.type === "status" && msg.peers === 1) {
        gotStatus = true;
        socket.close();
      }
    });
    socket.setTimeout(() => socket.close(), 2000);
  });

  check(wsRes, {
    "ws upgrade 101": (r) => r && r.status === 101,
    "received peers=1 status": () => gotStatus,
  });
}

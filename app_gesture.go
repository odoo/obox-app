package main

import (
	"encoding/json"
	"fmt"
)

func (a *App) getGestureScript() string {
	port := 4545
	if a.webserver != nil && a.webserver.Port > 0 {
		port = a.webserver.Port
	}
	wailsAppURL := a.getWailsAppURL()
	exitCorners := a.config.GetWebViewExitCorners()
	cornersJSON, err := json.Marshal(exitCorners)
	if err != nil {
		cornersJSON = []byte(`["top-right"]`)
	}

	return fmt.Sprintf(`(function() {
  if (window.__eposProxyExitInstalled) return;
  window.__eposProxyExitInstalled = true;

  var CORNER_SIZE = 140;
  var REQUIRED_TAPS = 4;
  var RESET_MS = 1000;
  var WAILS_APP_URL = %q;
  var PROXY_PORT = %d;
  var EXIT_CORNERS = %s;

  var tapCount = 0;
  var lastTapTime = 0;
  var activeCorner = null;

  function getTappedCorner(x, y) {
    var w = window.innerWidth || document.documentElement.clientWidth || (document.body ? document.body.clientWidth : 0);
    var h = window.innerHeight || document.documentElement.clientHeight || (document.body ? document.body.clientHeight : 0);
    var corner = null;
    if (x <= CORNER_SIZE && y <= CORNER_SIZE) {
      corner = "top-left";
    } else if (x >= w - CORNER_SIZE && y <= CORNER_SIZE) {
      corner = "top-right";
    } else if (x <= CORNER_SIZE && y >= h - CORNER_SIZE) {
      corner = "bottom-left";
    } else if (x >= w - CORNER_SIZE && y >= h - CORNER_SIZE) {
      corner = "bottom-right";
    }
    if (corner && EXIT_CORNERS && EXIT_CORNERS.indexOf(corner) !== -1) {
      return corner;
    }
    return null;
  }

  function flashIndicator(corner, count) {
    try {
      var dot = document.createElement("div");
      dot.style.position = "fixed";
      dot.style.width = "26px";
      dot.style.height = "26px";
      dot.style.borderRadius = "50%%";
      dot.style.backgroundColor = count >= REQUIRED_TAPS ? "#10B981" : "#EF4444";
      dot.style.zIndex = "2147483647";
      dot.style.pointerEvents = "none";
      dot.style.boxShadow = "0 0 10px rgba(0,0,0,0.5)";
      dot.style.transition = "opacity 0.4s";

      if (corner === "top-left") {
        dot.style.top = "12px"; dot.style.left = "12px";
      } else if (corner === "bottom-left") {
        dot.style.bottom = "12px"; dot.style.left = "12px";
      } else if (corner === "bottom-right") {
        dot.style.bottom = "12px"; dot.style.right = "12px";
      } else {
        // Default: top-right
        dot.style.top = "12px"; dot.style.right = "12px";
      }

      (document.body || document.documentElement).appendChild(dot);
      setTimeout(function() {
        dot.style.opacity = "0";
        setTimeout(function() { dot.remove(); }, 400);
      }, 350);
    } catch (e) {}
  }

  function handleTap(x, y) {
    var corner = getTappedCorner(x, y);
    if (!corner) {
      tapCount = 0;
      activeCorner = null;
      return false;
    }

    var now = Date.now();
    if (activeCorner === corner && (now - lastTapTime) < RESET_MS) {
      tapCount++;
    } else {
      activeCorner = corner;
      tapCount = 1;
    }
    lastTapTime = now;

    flashIndicator(corner, tapCount);

    if (tapCount >= REQUIRED_TAPS) {
      tapCount = 0;
      activeCorner = null;
      triggerExit();
      return true;
    }
    return false;
  }

  function triggerExit() {
    console.log("[ePOS] 4 taps detected on " + (activeCorner || "corner") + ", returning to Wails app");

    // 1. Direct top-level navigation to local proxy exit endpoint
    // Top-level navigation is never blocked by Mixed Content or CORS policies!
    try {
      window.location.replace("http://127.0.0.1:" + PROXY_PORT + "/api/kiosk/exit");
      return;
    } catch(e) {}

    // 2. Direct navigation to WAILS_APP_URL
    try {
      if (WAILS_APP_URL) {
        window.location.replace(WAILS_APP_URL + (WAILS_APP_URL.indexOf("?") >= 0 ? "&" : "?") + "kiosk_exit=1");
        return;
      }
    } catch(e) {}

    // 3. Fallback: beacon fetch
    try {
      fetch("http://127.0.0.1:" + PROXY_PORT + "/api/kiosk/exit", { method: "POST", mode: "no-cors" }).catch(function(){});
    } catch(e) {}
  }

  var lastInputTime = 0;
  function onInput(e) {
    if (e.button && e.button !== 0) return;
    var now = Date.now();
    if (now - lastInputTime < 60) return;
    lastInputTime = now;

    var clientX = e.clientX;
    var clientY = e.clientY;
    if (e.touches && e.touches.length > 0) {
      clientX = e.touches[0].clientX;
      clientY = e.touches[0].clientY;
    }
    if (typeof clientX !== "number" || typeof clientY !== "number") return;

    var triggered = handleTap(clientX, clientY);
    if (triggered) {
      try { e.preventDefault(); e.stopPropagation(); } catch (err) {}
    }
  }

  window.addEventListener("pointerdown", onInput, true);
  window.addEventListener("mousedown", onInput, true);
  window.addEventListener("touchstart", onInput, true);

  window.addEventListener("keydown", function(e) {
    if (e.key === "Escape" || (e.ctrlKey && e.altKey && e.key.toLowerCase() === "s") || e.key === "F11") {
      try { e.preventDefault(); } catch (err) {}
      triggerExit();
    }
  }, true);
})();`, wailsAppURL, port, string(cornersJSON))
}

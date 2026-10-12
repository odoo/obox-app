import React from "react";
import { createRoot } from "react-dom/client";
import App from "./App";
import "./style.css";

// Prevent default context menu globally
window.addEventListener("contextmenu", (e) => e.preventDefault(), true);
document.addEventListener("contextmenu", (e) => e.preventDefault(), true);

const container = document.getElementById("root");

const root = createRoot(container!);

root.render(
  <React.StrictMode>
    <App />
  </React.StrictMode>,
);

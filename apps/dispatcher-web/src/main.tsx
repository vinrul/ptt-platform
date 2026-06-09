import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import "maptalks/dist/maptalks.css";
import "./styles.css";
import { App } from "./app/App";

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <App />
  </StrictMode>,
);

import { printer } from "../../wailsjs/go/models";
import { sendLabelPrint } from "./sendLabelPrint";
import { sendEposPrint } from "./sendEposPrint";

export async function executePrint(
  printer: printer.Device,
  openCashDrawer = false,
) {
  if (printer.type === "label") {
    const response = await sendLabelPrint(printer);

    if (!response.ok) {
      if (response.status === 429) {
        throw new Error("Printer queue is full, please try again");
      }
      throw new Error(`Label print failed (HTTP ${response.status})`);
    }

    return;
  }

  const response = await sendEposPrint(printer, openCashDrawer);
  const xml = await response.text();
  const parser = new DOMParser();
  const doc = parser.parseFromString(xml, "text/xml");
  const responseEl = doc.querySelector("response");

  if (responseEl?.getAttribute("success") !== "true") {
    const code = responseEl?.getAttribute("code") || "Unknown error";
    if (code === "EX_BADPORT") {
      throw new Error(
        "The device is not connected, please check the printer power / connection",
      );
    }
    throw new Error(code);
  }
}

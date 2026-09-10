import { printer } from "../../wailsjs/go/models";

export async function sendLabelPrint(printer: printer.Device) {
  const zpl = `
^XA
^PW800
^LL250
^CF0,35
^FO0,40^FB800,1,0,C^FDTEST PRINT^FS
^CF0,25
^FO0,100^FB800,1,0,C^FD${printer.name}^FS
^XZ`;

  return await fetch(`http://${printer.ip}/pstprnt`, {
    method: "POST",
    headers: { "Content-Type": "application/octet-stream" },
    body: new TextEncoder().encode(zpl),
  });
}

import { printer } from "../../wailsjs/go/models";

export async function sendEposPrint(
  printer: printer.Device,
  openCashDrawer = false,
) {
  const content = openCashDrawer
    ? "<pulse />"
    : `
        <text font="font_e" em="true"/>
        <text align="center">This is a test receipt ${printer.name}</text>
        <feed line="3" />
        <cut type="feed" />
      `;

  return await fetch(`http://${printer.ip}/cgi-bin/epos/service.cgi`, {
    method: "POST",
    body: `<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/">
          <s:Body>
            <epos-print xmlns="http://www.epson-pos.com/schemas/2011/03/epos-print">
            ${content}
            </epos-print>
          </s:Body>
        </s:Envelope>`,
  });
}

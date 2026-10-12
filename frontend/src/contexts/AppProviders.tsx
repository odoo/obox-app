import { ReactNode } from "react";
import { ToastContextWrapper } from "./ToastContext";
import { AppContextWrapper } from "./AppContext";
import { PrinterContextWrapper } from "./PrinterContext";
import { OdooContextWrapper } from "./OdooContext";
import { WebViewContextWrapper } from "./WebViewContext";
import { PINContextWrapper } from "./PINContext";

export default function AppProviders({ children }: { children: ReactNode }) {
  return (
    <ToastContextWrapper>
      <AppContextWrapper>
        <WebViewContextWrapper>
          <PINContextWrapper>
            <PrinterContextWrapper>
              <OdooContextWrapper>{children}</OdooContextWrapper>
            </PrinterContextWrapper>
          </PINContextWrapper>
        </WebViewContextWrapper>
      </AppContextWrapper>
    </ToastContextWrapper>
  );
}

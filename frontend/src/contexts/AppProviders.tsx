import { ReactNode } from "react";
import { ToastContextWrapper } from "./ToastContext";
import { AppContextWrapper } from "./AppContext";
import { UpdateContextWrapper } from "./UpdateContext";
import { PrinterContextWrapper } from "./PrinterContext";
import { OdooContextWrapper } from "./OdooContext";

export default function AppProviders({ children }: { children: ReactNode }) {
  return (
    <ToastContextWrapper>
      <AppContextWrapper>
        <UpdateContextWrapper>
          <PrinterContextWrapper>
            <OdooContextWrapper>{children}</OdooContextWrapper>
          </PrinterContextWrapper>
        </UpdateContextWrapper>
      </AppContextWrapper>
    </ToastContextWrapper>
  );
}

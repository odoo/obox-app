import { useContext, useMemo } from "react";
import { AppContext } from "../contexts/AppContext";
import { brewSteps, linuxSteps, zadigSteps } from "../assets/data/fixStep";
import type { Step } from "../types";
import StepDialog from "./StepDialog";

export default function LibusbFixDialog({ printerName }: { printerName: string; }) {
  const appContext = useContext(AppContext);
  const os = appContext.data.system.os;
  const fixSteps = useMemo<Step[]>(() => {
    if (os === "windows") {
      return zadigSteps(printerName);
    }
    if (os === "darwin") {
      return brewSteps(printerName);
    }
    if (os === "linux") {
      return linuxSteps(printerName);
    }
    return [];
  }, [printerName, os]);

  if (fixSteps.length === 0) {
    return null;
  }
  const title = "Install Printer Driver";
  return (
    <StepDialog
      steps={fixSteps}
      title={title}
      openButton={
        <div className="mt-6 text-center">
          <div className="border border-transparent bg-odoo text-white hover:bg-odoo-dark rounded-lg px-4 py-2 cursor-pointer mt-2 text-sm font-medium">
            {`Fix - ${title}`}
          </div>
        </div>
      }
    />
  );
}

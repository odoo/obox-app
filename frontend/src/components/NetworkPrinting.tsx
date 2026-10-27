import { useContext, useEffect, useMemo, useState } from "react";
import StepDialog from "./StepDialog";
import { GetTroubleshootInfo } from "@bindings/obox-app/app";
import * as main from "@bindings/obox-app/models";
import { getLinuxSteps, getMacSteps, getWindowsSteps } from "../assets/data/troubleshootStep";
import type { Step } from "../types";
import { AppContext } from "../contexts/AppContext";

export default function NetworkPrinting() {
  const appContext = useContext(AppContext);
  const [isLoadingInfo, setIsLoadingInfo] = useState(false);
  const [info, setInfo] = useState<main.TroubleshootInfo | null>(null);

  const enabled = appContext.data.settings.networkPrintingEnabled;
  const os = appContext.data.system.os;

  const fetchInfo = async () => {
    setIsLoadingInfo(true);
    try {
      const data = await GetTroubleshootInfo();
      if (data) {
        setInfo(data);
      }
    } catch (err) {
      console.error("Failed to load troubleshoot info", err);
    } finally {
      setIsLoadingInfo(false);
    }
  };

  useEffect(() => {
    if (enabled) {
      fetchInfo();
    }
  }, [enabled]);

  const steps = useMemo<Step[]>(() => {
    if (!info) return [];
    if (os === "windows") {
      return getWindowsSteps(info);
    }
    if (os === "linux") {
      return getLinuxSteps(info);
    }
    if (os === "darwin") {
      return getMacSteps(info);
    }
    return [];
  }, [info, os]);

  if (!enabled) {
    return null;
  }

  return (
    <div className="mt-6 flex items-center justify-center">
      <StepDialog
        title="Troubleshoot Network Printing"
        steps={steps}
        isLoading={isLoadingInfo && !info}
        onOpen={fetchInfo}
        openButton={
          <span className="text-sm text-gray-600 hover:text-odoo cursor-pointer transition-colors">
            Having trouble printing?
          </span>
        }
      />
    </div>
  );
}

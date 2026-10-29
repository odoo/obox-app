import { useContext } from "react";
import { AppContext } from "../../contexts/AppContext";
import { useUpdate } from "../../contexts/UpdateContext";
import Dialog from "../Dialog";
import SystemInfo from "./SystemInfo";
import UpdateStatus from "./UpdateStatus";

export default function AboutDialog() {
  const appContext = useContext(AppContext);
  const { aboutSignal, build: { version }, isLoading } = appContext.data;
  const { data: { info }, setters: { setInfo } } = useUpdate();

  const handleClose = () => {
    if (info.state === "ready") {
      setInfo({ ...info, seen: false });
    }
  };

  return (
    <>
      <div className="fixed bottom-0.5 left-0.5 sm:bottom-1 sm:left-1 z-20 pointer-events-none select-none text-xs font-mono text-gray-400">
        {isLoading ? (
          <span className="inline-block w-8 h-2.5 bg-gray-200 rounded animate-pulse" />
        ) : (version)}
      </div>

      <Dialog
        title="About Obox App"
        maxWidth="max-w-md"
        showTitleDivider={true}
        openSignal={aboutSignal}
        onClose={handleClose}
      >
        <div className="space-y-4">
          <SystemInfo />
          <UpdateStatus />
        </div>
      </Dialog>
    </>
  );
}

export { SystemInfo, UpdateStatus };

import { useUpdate } from "../contexts/UpdateContext";
import UpdateStatus from "./about/UpdateStatus";

export default function UpdateBanner() {
  const {  data: { info },  actions: { handleDismiss }} = useUpdate();

  const downloadOrInstallFailed =
    info.state === "failed" &&
    !!info.latestVersion &&
    info.latestVersion !== info.currentVersion;

  const showBanner =
    !info.seen &&
    (["update_available", "downloading", "installing", "installed"].includes(info.state) || downloadOrInstallFailed);

  if (!showBanner) {
    return null;
  }

  return (
    <div className="animate-fade-in fixed bottom-4 right-4 z-40 w-full max-w-sm rounded-xl shadow-xl">
      <UpdateStatus onDismiss={handleDismiss} />
    </div>
  );
}
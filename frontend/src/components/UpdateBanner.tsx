import { useUpdate } from "../contexts/UpdateContext";
import UpdateStatus from "./about/UpdateStatus";

export default function UpdateBanner() {
  const {  data: { info },  actions: { handleDismiss }} = useUpdate();

  const downloadOrInstallFailed =
    info.state === "error" &&
    !!info.latestVersion &&
    info.latestVersion !== info.currentVersion;

  const showBanner =
    !info.seen &&
    (["available", "downloading", "installing", "ready"].includes(info.state) || downloadOrInstallFailed);

  if (!showBanner) {
    return null;
  }

  return (
    <div className="animate-fade-in fixed bottom-4 right-4 z-40 w-full max-w-sm rounded-xl shadow-xl">
      <UpdateStatus onDismiss={handleDismiss} />
    </div>
  );
}
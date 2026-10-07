import HeaderMenu from "./HeaderMenu";
import headerIcon from "@build/appicon.png";

export default function HeaderBar() {
    return (
        <header className="shrink-0 w-full px-4 sm:px-6 py-2.5 flex justify-between">
            <div className="flex items-center gap-2.5">
                <img
                    src={headerIcon}
                    alt="Obox App"
                    className="h-7 sm:h-8 w-auto object-contain"
                />
                <span className="text-lg font-bold tracking-tight select-none flex items-center gap-1">
                    <span className="text-odoo">Obox</span>
                    <span className="text-amber-500">App</span>
                </span>
            </div>

            <div className="flex items-center gap-2">
                <HeaderMenu />
            </div>
        </header>
    );
}

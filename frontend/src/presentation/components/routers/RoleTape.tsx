import type { RouterRole } from "@/domain/entities";
import { ROLE_TAPE } from "./routerRole";
import "./faceplate/faceplate.css";
import "./faceplate/rack.css";

/** A router's role on a strip of label-maker tape, as on its rack unit. */
export function RoleTape({ role }: { role: RouterRole }) {
  return (
    <span className={`fp-roletape fp-roletape-${role}`}>{ROLE_TAPE[role]}</span>
  );
}

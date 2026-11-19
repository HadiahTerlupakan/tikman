import { Alert, Empty, Typography } from "antd";
import { type CSSProperties, useMemo, useState } from "react";
import type {
  MikrotikRouter,
  RouterBgpSession,
  RouterInterface,
} from "@/domain/entities";
import {
  useRouterBgpSessions,
  useRouterInterfaces,
} from "@/application/hooks/useMikrotikRouters";
import { healthView } from "./healthModel";
import { HealthStrip } from "./HealthStrip";
import { PortDetailPanel } from "./PortDetailPanel";
import { PortLabels } from "./PortLabels";
import {
  layoutFaceplate,
  physicalPortNames,
  type FaceplateLayout,
} from "./portLayout";
import {
  bgpStale,
  buildPortViews,
  routerReachable,
  type PortView,
} from "./portModel";

// The labels shrink to fit; only past their readable floor does a board scroll here instead of widening the page.
const STAGE: CSSProperties = {
  border: "1px solid var(--fp-stage-line)",
  borderRadius: 10,
  overflowX: "auto",
};

// Stable fallbacks: a fresh [] per render would rebuild the port views, and the
// label layout effect that depends on them would loop while a query loads.
const NO_INTERFACES: RouterInterface[] = [];
const NO_SESSIONS: RouterBgpSession[] = [];

/** The operator's pick while that port is still in use; otherwise the first trunk, else the first used port. */
function selectedPort(
  layout: FaceplateLayout,
  ports: Map<string, PortView>,
  picked: string | undefined,
): string | undefined {
  const used = layout.jacks
    .map((j) => j.name)
    .filter((name) => ports.get(name)?.role != null);
  if (picked && used.includes(picked)) return picked;
  return used.find((name) => ports.get(name)?.role === "trunk") ?? used[0];
}

/** The router's front panel with cable labels, and the panel of the port selected on it. */
export function RouterFaceplate({ router }: { router: MikrotikRouter }) {
  const {
    data: interfaces = NO_INTERFACES,
    isLoading,
    isError,
  } = useRouterInterfaces(router.id);
  const { data: sessions = NO_SESSIONS } = useRouterBgpSessions(router.id);
  const [picked, setPicked] = useState<string>();
  const physical = useMemo(() => physicalPortNames(interfaces), [interfaces]);
  const layout = useMemo(
    () => layoutFaceplate(router.boardName, physical),
    [router.boardName, physical],
  );
  const ports = useMemo(
    () => buildPortViews(router, interfaces, sessions),
    [router, interfaces, sessions],
  );

  if (isLoading) return null;
  if (isError)
    return <Alert type="error" message="Data interface gagal dimuat" />;
  if (physical.length === 0) {
    return <Empty description="Belum ada data interface" />;
  }
  const offline = !routerReachable(router);
  const selected = selectedPort(layout, ports, picked);
  const view = selected ? ports.get(selected) : undefined;
  const health = healthView(router, new Date());
  return (
    <div style={{ marginBottom: 16 }}>
      {!layout.templated && (
        <Typography.Paragraph type="secondary">
          {`Model ${layout.model || "ini"} belum punya cetakan, jadi susunan port dibuat otomatis dari nama interface.`}
        </Typography.Paragraph>
      )}
      <div style={STAGE}>
        <PortLabels
          layout={layout}
          ports={ports}
          offline={offline}
          health={health}
          selected={selected}
          onSelect={setPicked}
        />
      </div>
      <HealthStrip health={health} healthAt={router.healthAt} />
      {view && (
        <PortDetailPanel
          routerId={router.id}
          view={view}
          offline={offline}
          stale={bgpStale(router)}
        />
      )}
    </div>
  );
}

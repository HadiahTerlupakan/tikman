import { Tag } from "antd";
import type { MikrotikRouter } from "@/domain/entities";

export function RouterStatusTag({ router }: { router: MikrotikRouter }) {
  if (!router.enabled) return <Tag>Nonaktif</Tag>;
  if (router.status === "down")
    return <Tag color="red">SNMP tidak menjawab</Tag>;
  if (router.apiStatus === "certificate_changed")
    return <Tag color="orange">Sertifikat berubah</Tag>;
  if (router.apiStatus === "error") return <Tag color="orange">API error</Tag>;
  if (router.status === "up") return <Tag color="green">Up</Tag>;
  return <Tag>Belum terbaca</Tag>;
}

import { Alert, Skeleton, Typography } from "antd";
import type { ZteGPONRegisterRequest } from "@/domain/entities";
import { useZteCommandPreview } from "@/application/hooks/useZteProvisioning";

interface ZteCommandPreviewProps {
  mode: "register" | "configure";
  targetId?: string;
  request: ZteGPONRegisterRequest;
}

// The commands come from the server, which owns the builder that runs them.
// This used to assemble its own copy, and that copy drifted: it kept a keyword
// the OLT rejects, put the WAN lines in the wrong context, and showed a
// placeholder ONU ID instead of the one the allocator assigns.
export function ZteCommandPreview({
  mode,
  targetId,
  request,
}: ZteCommandPreviewProps) {
  const { data, isLoading, error } = useZteCommandPreview(
    mode,
    targetId,
    request,
    true,
  );

  if (isLoading) {
    return <Skeleton active paragraph={{ rows: 6 }} />;
  }

  if (error) {
    return (
      <Alert
        type="error"
        showIcon
        message="The OLT would refuse this configuration"
        description={error.message}
      />
    );
  }

  const commands = data?.commands ?? [];

  return (
    <Alert
      type="info"
      showIcon
      message={
        data?.onuId
          ? `Command preview — ONU ID ${data.onuId} (password redacted)`
          : "Command preview (password redacted)"
      }
      description={
        <Typography.Paragraph copyable={{ text: commands.join("\n") }}>
          <pre style={{ whiteSpace: "pre-wrap", margin: 0 }}>
            {commands.join("\n")}
          </pre>
        </Typography.Paragraph>
      }
    />
  );
}

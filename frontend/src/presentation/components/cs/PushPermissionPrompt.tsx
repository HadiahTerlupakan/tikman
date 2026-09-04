import { useState } from "react";
import { Button, Modal, Typography } from "antd";
import { BellOutlined } from "@ant-design/icons";
import type { PushPermission } from "@/infrastructure/firebase/messaging";

const { Paragraph } = Typography;

/** Remembers a "nanti saja" for the rest of the tab's life. sessionStorage,
 * not localStorage: a CS who puts it off should be asked again next shift, not
 * never again on that machine. */
const DISMISSED_KEY = "tikman.push-prompt-dismissed";

interface PushPermissionPromptProps {
  permission: PushPermission;
  requesting: boolean;
  onEnable: () => void;
}

/**
 * Asks to turn notifications on when the app opens, so nobody has to find a
 * button — but asks with a dialog rather than firing the browser's own prompt
 * on load.
 *
 * That is not politeness, it is the only thing that works: Firefox and Safari
 * require a user gesture for Notification.requestPermission(), so a call on
 * mount resolves without ever showing anything, and Chrome demotes sites that
 * auto-prompt to a quieter UI. A permission that gets refused can never be
 * asked for again, so spending it on a prompt half the browsers will not even
 * display is the expensive mistake here. The click on "Aktifkan" IS the
 * gesture.
 */
export function PushPermissionPrompt({
  permission,
  requesting,
  onEnable,
}: PushPermissionPromptProps) {
  const [dismissed, setDismissed] = useState(
    () => sessionStorage.getItem(DISMISSED_KEY) === "1",
  );

  // "default" is the only state worth asking in: granted is done, denied can
  // only be undone in browser settings, and unsupported means there is nothing
  // to turn on.
  const open = permission === "default" && !dismissed;

  const dismiss = () => {
    setDismissed(true);
    try {
      sessionStorage.setItem(DISMISSED_KEY, "1");
    } catch {
      // Private browsing can refuse storage. Losing the preference only means
      // asking again on the next page load, which is not worth failing over.
    }
  };

  return (
    <Modal
      open={open}
      onCancel={dismiss}
      title={
        <span>
          <BellOutlined style={{ marginInlineEnd: 8 }} />
          Aktifkan notifikasi?
        </span>
      }
      footer={[
        <Button key="later" onClick={dismiss}>
          Nanti saja
        </Button>,
        <Button
          key="enable"
          type="primary"
          loading={requesting}
          onClick={onEnable}
        >
          Aktifkan
        </Button>,
      ]}
    >
      <Paragraph style={{ marginBottom: 0 }}>
        Dapatkan pemberitahuan begitu pelanggan mengirim pesan, tanpa harus
        membuka CS Inbox untuk mengeceknya. Browser akan meminta izin sekali
        setelah ini.
      </Paragraph>
    </Modal>
  );
}

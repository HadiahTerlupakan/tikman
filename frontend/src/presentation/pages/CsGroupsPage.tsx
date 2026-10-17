import { PageHeader } from "@/presentation/components/common";
import { CreateGroupModal } from "@/presentation/components/cs/groups/CreateGroupModal";
import { GroupInfoPane } from "@/presentation/components/cs/groups/GroupInfoPane";
import { GroupList } from "@/presentation/components/cs/groups/GroupList";
import { GroupListToolbar } from "@/presentation/components/cs/groups/GroupListToolbar";
import { GroupThreadPane } from "@/presentation/components/cs/groups/GroupThreadPane";
import { InviteLinkModal } from "@/presentation/components/cs/groups/InviteLinkModal";
import { fullHeightPage } from "@/presentation/components/layout/layoutPadding";
import { colors } from "@/shared/theme/colors";
import { useCsGroups } from "./useCsGroups";

const panel = {
  background: colors.surface,
  border: `1px solid ${colors.border}`,
  borderRadius: 10,
  overflow: "hidden",
  display: "flex",
  flexDirection: "column",
} as const;

/**
 * WhatsApp groups, on a page of their own. Kept out of CS Inbox on purpose: a
 * busy group would push the customers who are waiting on an answer off the
 * top of the list.
 */
export function CsGroupsPage() {
  const g = useCsGroups();
  const paneWidth = (column: number) =>
    g.layout.columns ? { width: column } : { flex: 1, minWidth: 0 };
  // Labels only matter once there is more than one number to tell apart.
  const accountLabels =
    g.accounts.length > 1
      ? Object.fromEntries(g.accounts.map((a) => [a.id, a.label]))
      : undefined;

  return (
    <div
      style={{
        display: "flex",
        flexDirection: "column",
        height: fullHeightPage(g.screens),
      }}
    >
      <PageHeader
        title="Grup WA"
        description="Grup WhatsApp dari setiap nomor yang tersambung"
      />

      <div style={{ display: "flex", gap: 12, flex: 1, minHeight: 0 }}>
        {g.showsPane("list") && (
          <div style={{ ...panel, ...paneWidth(340) }}>
            <GroupListToolbar
              search={g.search}
              onSearchChange={g.setSearch}
              accounts={g.accounts}
              waAccountId={g.waAccountId}
              onAccountChange={g.setAccount}
              refreshing={g.refresh.isPending}
              onRefresh={() => g.refresh.mutate(g.waAccountId)}
              onCreate={g.isAdmin ? () => g.setCreateOpen(true) : undefined}
            />
            <div style={{ flex: 1, overflowY: "auto" }}>
              <GroupList
                groups={g.groups}
                selectedId={g.selectedId}
                accountLabels={accountLabels}
                onSelect={g.handleSelect}
              />
            </div>
          </div>
        )}

        {g.showsPane("thread") && (
          <div style={{ ...panel, flex: 1, minWidth: 0 }}>
            <GroupThreadPane
              group={g.selected}
              messages={g.history.messages}
              loading={g.history.isLoading}
              hasOlder={!!g.history.hasNextPage}
              loadingOlder={g.history.isFetchingNextPage}
              onLoadOlder={() => g.history.fetchNextPage()}
              replyTo={g.replyTo}
              sending={g.sendMessage.isPending}
              attaching={g.sendMedia.isPending}
              participants={g.detailQuery.data?.participants ?? []}
              onSend={g.handleSend}
              onAttach={g.handleAttach}
              onReply={g.setReplyTo}
              onCancelReply={() => g.setReplyTo(undefined)}
              onBack={g.layout.columns ? undefined : () => g.navigate(-1)}
              onOpenInfo={
                g.layout.columns
                  ? undefined
                  : () => g.patchParams({ panel: "info" }, true)
              }
            />
          </div>
        )}

        {g.showsPane("customer") && (
          <div style={{ ...panel, ...paneWidth(320) }}>
            <GroupInfoPane
              group={g.selected}
              participants={g.detailQuery.data?.participants ?? []}
              loading={g.detailQuery.isLoading}
              isAdmin={g.isAdmin}
              busy={g.acting}
              onAction={(request) =>
                g.selected && g.runAction(g.selected.id, request)
              }
              onBack={g.layout.columns ? undefined : () => g.navigate(-1)}
            />
          </div>
        )}
      </div>

      {g.isAdmin && (
        <CreateGroupModal
          open={g.createOpen}
          accounts={g.accounts}
          creating={g.creating}
          onCreate={g.createGroup}
          onClose={() => g.setCreateOpen(false)}
        />
      )}
      <InviteLinkModal link={g.inviteLink} onClose={g.closeInviteLink} />
    </div>
  );
}

export default CsGroupsPage;

import type {
  WaGroup,
  WaGroupAction,
  WaGroupActionRequest,
  WaGroupDetail,
  WaGroupFilter,
  WaGroupMessage,
  WaGroupPollRequest,
} from "../entities";

export interface IWaGroupRepository {
  list(filter?: WaGroupFilter): Promise<WaGroup[]>;
  get(groupId: string): Promise<WaGroupDetail>;
  /** Newest first. Reading also clears the group's unread count. */
  getHistory(
    groupId: string,
    limit?: number,
    offset?: number,
  ): Promise<WaGroupMessage[]>;
  sendMessage(
    groupId: string,
    body: string,
    replyToId?: string,
    mentions?: string[],
  ): Promise<WaGroupMessage>;
  sendMedia(
    groupId: string,
    file: File,
    caption?: string,
    replyToId?: string,
    mentions?: string[],
  ): Promise<WaGroupMessage>;
  refresh(waAccountId?: string): Promise<void>;
  create(
    waAccountId: string,
    name: string,
    participants: string[],
  ): Promise<WaGroupAction>;
  act(groupId: string, request: WaGroupActionRequest): Promise<WaGroupAction>;
  getAction(actionId: string): Promise<WaGroupAction>;
  setPinned(groupId: string, pinned: boolean): Promise<WaGroup>;
  /** Shows or clears our "sedang mengetik" to the group. */
  setTyping(groupId: string, typing: boolean): Promise<void>;
  /** Asks the phone for the messages before the oldest one stored. */
  requestHistory(groupId: string): Promise<WaGroupAction>;
  /** Newest first. */
  getStarred(groupId: string): Promise<WaGroupMessage[]>;
  createPoll(
    groupId: string,
    poll: WaGroupPollRequest,
  ): Promise<WaGroupMessage>;
}

import type {
  WaGroupAction,
  WaGroupMessage,
  WaGroupReceipt,
} from "../entities";

/** What can be done to one message already in a group. */
export interface IWaGroupMessageRepository {
  /** "" removes our reaction. */
  react(
    groupId: string,
    messageId: string,
    emoji: string,
  ): Promise<WaGroupAction>;
  edit(
    groupId: string,
    messageId: string,
    body: string,
  ): Promise<WaGroupAction>;
  /** Deletes it for every member. */
  revoke(groupId: string, messageId: string): Promise<WaGroupAction>;
  setStarred(
    groupId: string,
    messageId: string,
    starred: boolean,
  ): Promise<WaGroupMessage>;
  forward(
    groupId: string,
    messageId: string,
    groupIds: string[],
  ): Promise<WaGroupMessage[]>;
  /** Opens the sender's 1:1 thread in the inbox and answers its id. */
  replyPrivately(groupId: string, messageId: string): Promise<string>;
  getReceipts(groupId: string, messageId: string): Promise<WaGroupReceipt[]>;
  /** Option names; [] withdraws the vote. */
  vote(
    groupId: string,
    messageId: string,
    options: string[],
  ): Promise<WaGroupAction>;
}

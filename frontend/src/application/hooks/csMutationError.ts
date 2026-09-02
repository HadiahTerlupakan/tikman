import { message } from "antd";

/**
 * Every CS mutation reports a failure the same way, and what it reports is the
 * server's own sentence: mapApiError already puts the API's `error` field on
 * Error.message, and the 409 naming who holds a thread is the one error this
 * module wrote a human explanation for. Swallowing it left the CS with a
 * reply that had vanished and no reason why.
 */
export function reportCsMutationError(error: Error) {
  message.error(error.message);
}

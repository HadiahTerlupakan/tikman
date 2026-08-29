import { describe, expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { UserRole, type User } from "@/domain/entities";
import { UserModal } from "./UserModal";

const admin: User = {
  id: "user-1",
  username: "admin",
  email: "admin@tikman.local",
  role: UserRole.ADMIN,
} as User;

const renderModal = (user?: User) => {
  const onSubmit = vi.fn();
  render(
    <UserModal
      open
      user={user}
      onClose={vi.fn()}
      onSubmit={onSubmit}
      loading={false}
    />,
  );
  return onSubmit;
};

const passwordFields = () =>
  screen.getAllByLabelText(/password/i) as HTMLInputElement[];

describe("UserModal", () => {
  it("offers a password field when editing, so a password can be changed at all", async () => {
    // The field used to be rendered only when creating a user, which left no
    // way to change one from the interface.
    renderModal(admin);

    expect(screen.getByLabelText("Password")).toBeInTheDocument();
    expect(screen.getByText(/leave blank to keep/i)).toBeInTheDocument();
  });

  it("leaves the password out entirely when the field is blank on edit", async () => {
    const onSubmit = renderModal(admin);

    await userEvent.click(screen.getByRole("button", { name: /ok/i }));

    await waitFor(() => expect(onSubmit).toHaveBeenCalled());
    const submitted = onSubmit.mock.calls[0][0];
    // Sending an empty string would fail the API's minimum length and read to
    // the operator as a rejected edit.
    expect("password" in submitted).toBe(false);
    expect(submitted).toMatchObject({
      username: "admin",
      role: UserRole.ADMIN,
    });
  });

  it("rejects a password shorter than the API accepts", async () => {
    const onSubmit = renderModal(admin);

    await userEvent.type(screen.getByLabelText("Password"), "short123");
    await userEvent.click(screen.getByRole("button", { name: /ok/i }));

    expect(
      await screen.findByText(/at least 12 characters/i),
    ).toBeInTheDocument();
    expect(onSubmit).not.toHaveBeenCalled();
  });

  it("refuses a confirmation that does not match", async () => {
    const onSubmit = renderModal(admin);

    const [password, confirm] = passwordFields();
    await userEvent.type(password, "correct-horse-battery");
    await userEvent.type(confirm, "correct-horse-batteryX");
    await userEvent.click(screen.getByRole("button", { name: /ok/i }));

    expect(await screen.findByText(/do not match/i)).toBeInTheDocument();
    expect(onSubmit).not.toHaveBeenCalled();
  });

  it("submits the password once it is typed twice the same", async () => {
    const onSubmit = renderModal(admin);

    const [password, confirm] = passwordFields();
    await userEvent.type(password, "correct-horse-battery");
    await userEvent.type(confirm, "correct-horse-battery");
    await userEvent.click(screen.getByRole("button", { name: /ok/i }));

    await waitFor(() => expect(onSubmit).toHaveBeenCalled());
    expect(onSubmit.mock.calls[0][0]).toMatchObject({
      password: "correct-horse-battery",
    });
    // The confirmation is a form-only field and must not reach the API.
    expect("passwordConfirm" in onSubmit.mock.calls[0][0]).toBe(false);
  });
});

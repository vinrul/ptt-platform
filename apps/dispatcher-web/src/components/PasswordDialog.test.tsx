import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { PasswordDialog } from "./PasswordDialog";

afterEach(cleanup);

describe("PasswordDialog", () => {
  it("validates confirmation and submits the passwords", async () => {
    const onSubmit = vi.fn(async () => {});
    render(
      <PasswordDialog
        description="Change password"
        onClose={() => {}}
        onSubmit={onSubmit}
        open
        requireCurrentPassword
        submitLabel="Save"
        title="Change password"
      />,
    );

    fireEvent.change(screen.getByLabelText("Current password"), {
      target: { value: "old-password" },
    });
    fireEvent.change(screen.getByLabelText("New password"), {
      target: { value: "new-password" },
    });
    fireEvent.change(screen.getByLabelText("Confirm new password"), {
      target: { value: "different-password" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    expect(await screen.findByText("Konfirmasi password tidak sama.")).toBeInTheDocument();
    expect(onSubmit).not.toHaveBeenCalled();

    fireEvent.change(screen.getByLabelText("Confirm new password"), {
      target: { value: "new-password" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));

    await waitFor(() => {
      expect(onSubmit).toHaveBeenCalledWith("old-password", "new-password");
    });
  });

  it("can show and hide password fields", () => {
    render(
      <PasswordDialog
        description="Reset password"
        onClose={() => {}}
        onSubmit={async () => {}}
        open
        submitLabel="Reset"
        title="Reset password"
      />,
    );

    expect(screen.getByLabelText("New password")).toHaveAttribute("type", "password");
    fireEvent.click(screen.getByRole("checkbox", { name: "Show password" }));
    expect(screen.getByLabelText("New password")).toHaveAttribute("type", "text");
  });
});

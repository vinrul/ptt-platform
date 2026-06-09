import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { LoginPage } from "./LoginPage";
import { useAuthStore } from "./authStore";

vi.mock("../../lib/api", () => ({
  login: vi.fn(async () => ({
    accessToken: "access-token",
    refreshToken: "refresh-token",
    user: {
      id: "dispatcher-1",
      username: "dispatcher1",
      fullName: "Dispatcher One",
      role: "dispatcher",
      status: "active",
    },
  })),
}));

describe("LoginPage", () => {
  beforeEach(() => {
    window.localStorage.clear();
    useAuthStore.setState({ session: null });
  });

  it("logs in an operator account", async () => {
    render(<LoginPage />);

    fireEvent.change(screen.getByLabelText("Username"), {
      target: { value: "dispatcher1" },
    });
    fireEvent.change(screen.getByLabelText("Password"), {
      target: { value: "password" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Masuk ke console" }));

    await waitFor(() => {
      expect(useAuthStore.getState().session?.user.role).toBe("dispatcher");
    });
  });
});

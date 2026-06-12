import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { UserList } from "./UserList";

describe("UserList", () => {
  it("renders online users before offline users", () => {
    render(
      <UserList
        presence={{
          "user-2": { status: "online", lastSeenAt: "2026-06-09T00:00:00Z" },
        }}
        users={[
          {
            id: "user-1",
            username: "field1",
            fullName: "Alpha Field",
            role: "field_user",
            status: "active",
          },
          {
            id: "user-2",
            username: "field2",
            fullName: "Bravo Field",
            role: "field_user",
            status: "active",
          },
        ]}
      />,
    );

    const names = screen.getAllByText(/Field$/).map((element) => element.textContent);
    expect(names).toEqual(["Bravo Field", "Alpha Field"]);
  });

  it("does not render per-user GPS history actions", () => {
    render(
      <UserList
        presence={{}}
        users={[
          {
            id: "user-1",
            username: "field1",
            fullName: "Alpha Field",
            role: "field_user",
            status: "active",
          },
        ]}
      />,
    );

    expect(screen.queryByRole("button", { name: /history/i })).not.toBeInTheDocument();
  });
});

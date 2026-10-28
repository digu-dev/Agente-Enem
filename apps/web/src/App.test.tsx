import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { App } from "./App";

describe("App", () => {
  it("renders the ENEM landing page", () => {
    render(<App />);

    expect(
      screen.getByRole("heading", { name: "ENEM Platform" }),
    ).toBeVisible();
    expect(
      screen.getByText("Seu espaço de preparação para o ENEM."),
    ).toBeVisible();
  });
});

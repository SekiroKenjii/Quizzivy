import "@/lib/i18n";
import { render, screen } from "@testing-library/react";
import { ContentImage } from "@/components/shared/content/ContentImage";

// D-065: a prompt or explanation image may carry an empty alt, which means
// decorative (D-064.2); the shared renderer must draw it as alt="" so a screen
// reader announces nothing, while a described image keeps its alt.
describe("ContentImage alt text", () => {
  it("draws an empty alt as a decorative image with nothing announced", () => {
    const { container } = render(
      <ContentImage src="https://media.test/a.png" alt="" />,
    );
    const img = container.querySelector("img");
    expect(img).not.toBeNull();
    expect(img?.getAttribute("alt")).toBe("");
    expect(screen.queryByRole("img")).toBeNull();
    expect(screen.getByRole("presentation")).toBe(img);
  });

  it("announces a described image by its alt", () => {
    render(<ContentImage src="https://media.test/b.png" alt="A map of Hanoi" />);
    expect(screen.getByRole("img", { name: "A map of Hanoi" })).toBeInTheDocument();
  });
});

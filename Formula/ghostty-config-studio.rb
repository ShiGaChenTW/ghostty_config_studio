class GhosttyConfigStudio < Formula
  desc "Terminal workbench for Ghostty: browse themes, build config files field by field"
  homepage "https://github.com/ShiGaChenTW/ghostty_config_studio"
  url "https://github.com/ShiGaChenTW/ghostty_config_studio/archive/refs/tags/v0.1.0.tar.gz"
  sha256 "REPLACE_WITH_RELEASE_SHA256"
  license "MIT"
  head "https://github.com/ShiGaChenTW/ghostty_config_studio.git", branch: "main"

  depends_on "go" => :build
  depends_on :macos

  # Every executable is installed into libexec and exposed through a generated
  # exec wrapper rather than a symlink. The shell entry points locate lib/ via
  # $BASH_SOURCE, which resolves to the symlink itself — so a symlinked
  # ghostty-theme would look for lib/menu.sh inside Homebrew's bin directory
  # and fail. The wrapper execs the real path, keeping $BASH_SOURCE honest.
  ENTRY_POINTS = %w[
    ghostty-setup ghostty-theme ghostty-font ghostty-preset ghostty-cursor
    ghostty-custom ghostty-window ghostty-cursor-style ghostty-clipboard
  ].freeze

  def install
    cd "tui" do
      # The reported version is injected from this formula's own `version`,
      # which Homebrew parses out of the tagged url above. It used to be a
      # hand-edited constant in main.go, and v0.1.9 shipped a binary still
      # calling itself 0.1.8 — the test block below compares the two, so the
      # only way to keep them honest is to have one source for both.
      system "go", "build", *std_go_args(output: libexec/"ghostty-tui",
                                         ldflags: "-X main.version=#{version}")
    end
    bin.write_exec_script libexec/"ghostty-tui"

    libexec.install "lib"
    ENTRY_POINTS.each do |script|
      libexec.install script
      bin.write_exec_script libexec/script
    end

    doc.install "README.md", "NOTICE.md", "DESIGN_NOTES.md"
  end

  def caveats
    <<~EOS
      Run the workbench with:
        ghostty-tui

      It works immediately — Ghostty's own 460+ themes and all 200 settings are
      read from your local Ghostty install, nothing else to download.

      Optional community theme packs (GLSL shaders, cursor effects):
        ghostty-setup

      Imported packs and your own configs live in
        ~/.config/ghostty-config-studio/
      so they survive `brew upgrade` and uninstall.
    EOS
  end

  test do
    # Point every writable path at the sandbox so the test never touches a real
    # Ghostty config, and assert the tool reports a clean empty state rather
    # than tripping over the absent asset directory.
    ENV["GHOSTTY_DIR"] = testpath/"ghostty"
    ENV["GHOSTTY_STUDIO_DIR"] = testpath/"studio"

    # Assertions match the strings that are identical in both languages, so
    # the test does not quietly depend on which one happens to be default.
    assert_match "ghostty-picker", shell_output("#{bin}/ghostty-theme --current")

    # No packs imported: the picker must decline cleanly, not crash.
    assert_match "ghostty-setup", shell_output("#{bin}/ghostty-theme 2>&1", 1)

    assert_match version.to_s, shell_output("#{bin}/ghostty-tui --version")
  end
end

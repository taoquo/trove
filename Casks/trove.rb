# Installed with: brew install --cask taoquo/trove/trove
#
# The download names are the three the release workflow always produces, so the cask
# follows the latest release instead of being edited for each version. That is also why
# sha256 is :no_check here: the release carries SHA256SUMS.txt and a build attestation,
# and a cask file that had to be updated by hand would be stale more often than correct.
cask "trove" do
  version :latest
  sha256 :no_check

  on_arm do
    url "https://github.com/taoquo/trove/releases/latest/download/trove-macos-arm64.dmg"
  end
  on_intel do
    url "https://github.com/taoquo/trove/releases/latest/download/trove-macos-x64.dmg"
  end

  name "trove"
  desc "Local-first library for your X bookmarks"
  homepage "https://trove.taoquo.com/"

  app "trove.app"

  # The builds are ad-hoc signed, so macOS asks before the first launch. Homebrew cannot
  # answer that dialog; the reader can.
  caveats <<~EOS
    trove is not notarized yet. On first launch, right-click trove in Applications and
    choose Open. Updates after that install from inside the app.
  EOS
end

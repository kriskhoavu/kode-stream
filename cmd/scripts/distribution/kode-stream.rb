class KodeStream < Formula
  desc "Git-native workspace for planning docs, Jira context, and terminals"
  homepage "https://github.com/kriskhoavu/kode-stream"

  # No `version` line: brew audit --strict rejects it as redundant with the
  # version scanned from the URL. update_formula.py rewrites the version inside
  # these URLs, so keep the version literal in the path rather than interpolated.
  if OS.mac? && Hardware::CPU.arm?
    url "https://github.com/kriskhoavu/kode-stream/releases/download/v0.0.0/kode-stream_0.0.0_darwin_arm64.tar.gz"
    sha256 "REPLACE_DARWIN_ARM64_SHA256"
  elsif OS.mac? && Hardware::CPU.intel?
    url "https://github.com/kriskhoavu/kode-stream/releases/download/v0.0.0/kode-stream_0.0.0_darwin_amd64.tar.gz"
    sha256 "REPLACE_DARWIN_AMD64_SHA256"
  elsif OS.linux? && Hardware::CPU.intel? && Hardware::CPU.is_64_bit?
    url "https://github.com/kriskhoavu/kode-stream/releases/download/v0.0.0/kode-stream_0.0.0_linux_amd64.tar.gz"
    sha256 "REPLACE_LINUX_AMD64_SHA256"
  else
    odie "kode-stream has no prebuilt binary for this platform"
  end

  def install
    bin.install "kode-stream"
  end

  test do
    # kode-stream prints its usage and exits 2 when invoked with no subcommand.
    output = shell_output("#{bin}/kode-stream 2>&1", 2)
    assert_match "Usage", output
    assert_match "kode-stream serve", output
  end
end

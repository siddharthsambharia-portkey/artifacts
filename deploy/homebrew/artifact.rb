class Artifact < Formula
  desc "Internal hosting platform — OSS Quick for every company"
  homepage "https://github.com/artifact/artifact"
  url "https://github.com/artifact/artifact/archive/refs/tags/v0.1.0.tar.gz"
  sha256 "SKIP"
  license "Apache-2.0"

  depends_on "go" => :build

  def install
    system "go", "build", "-o", bin/"artifact", "./cmd/artifact"
  end

  test do
    assert_match "0.1.0", shell_output("#{bin}/artifact version")
  end
end

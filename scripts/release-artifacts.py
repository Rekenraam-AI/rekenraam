#!/usr/bin/env python3
"""Package Linux server candidates and verify their signed file manifest (#333)."""
import argparse
import gzip
import hashlib
import io
import json
import os
from pathlib import Path
import re
import struct
import subprocess
import tarfile

ROOT = Path(__file__).resolve().parent.parent
VERSION = re.compile(r"v(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)-rc\.(0|[1-9]\d*)")
ARCHES = {"amd64": 62, "arm64": 183}
SIDECARS = {"SHA256SUMS", "SHA256SUMS.sigstore.json"}


def candidate_version(value):
    if not VERSION.fullmatch(value):
        raise ValueError("candidate version must be vMAJOR.MINOR.PATCH-rc.N")
    return value


def run(*args, cwd=ROOT):
    return subprocess.check_output(args, cwd=cwd, text=True).strip()


def sha256(data):
    return hashlib.sha256(data).hexdigest()


def go_build_info(binary):
    lines = run("go", "version", "-m", str(binary), cwd=ROOT / "backend").splitlines()
    # go version prints the inspected filename even for a -trimpath binary.
    # Keep metadata deterministic when the same executable moves between hosts.
    lines[0] = "rekenraam: " + lines[0].split(": ", 1)[1]
    return "\n".join(lines)


def migration_manifest(directory):
    files = sorted(directory.glob("*.sql"))
    versions = [int(p.name.split("_", 1)[0]) for p in files]
    if not files or versions != list(range(1, len(files) + 1)):
        raise ValueError("migration numbers must be contiguous from 1")
    text = "".join(f"{sha256(p.read_bytes())}  {p.name}\n" for p in files)
    return versions[-1], text


def inspect_binary(binary, arch):
    header = binary.read_bytes()[:20]
    if (len(header) != 20 or header[:6] != b"\x7fELF\x02\x01"
            or struct.unpack_from("<H", header, 18)[0] != ARCHES[arch]):
        raise ValueError(f"expected a 64-bit little-endian Linux {arch} executable")
    symbols = run("readelf", "--version-info", str(binary))
    glibc = sorted({m for m in re.findall(r"GLIBC_(\d+\.\d+(?:\.\d+)?)", symbols)},
                   key=lambda v: tuple(map(int, v.split("."))))
    if not glibc or tuple(map(int, glibc[-1].split("."))) > (2, 39):
        raise ValueError("binary must use glibc symbols no newer than 2.39")
    needed = sorted(re.findall(r"Shared library: \[([^]]+)\]",
                               run("readelf", "--dynamic", str(binary))))
    if "libc.so.6" not in needed or any("sqlite" in name for name in needed):
        raise ValueError("expected glibc with bundled SQLite, not system libsqlite3")
    return {"baseline": "Ubuntu 24.04 / glibc 2.39", "needed": needed,
            "highest_glibc_symbol": glibc[-1]}


def write_archive(path, prefix, files, epoch):
    """Stable gzip/tar headers; files are bytes and explicit executable flags."""
    with path.open("xb") as output:
        with gzip.GzipFile(filename="", mode="wb", fileobj=output, mtime=epoch) as compressed:
            with tarfile.open(mode="w", fileobj=compressed, format=tarfile.USTAR_FORMAT) as archive:
                for name, (data, executable) in sorted(files.items()):
                    entry = tarfile.TarInfo(f"{prefix}/{name}")
                    entry.size, entry.mtime = len(data), epoch
                    entry.mode = 0o755 if executable else 0o644
                    archive.addfile(entry, io.BytesIO(data))


def package(args):
    version = candidate_version(args.version)
    if run("git", "status", "--porcelain", "--untracked-files=no"):
        raise ValueError("commit tracked changes before packaging a candidate")
    binary = args.binary.resolve()
    source = run("git", "rev-parse", "HEAD")
    build = go_build_info(binary)
    if f"vcs.revision={source}" not in build or "vcs.modified=false" not in build:
        raise ValueError("binary must identify the current clean source commit")
    if "CGO_ENABLED=1" not in build or "-trimpath=true" not in build:
        raise ValueError("binary must use CGO and -trimpath")
    if not all(tag in build for tag in ("fts5", "netgo", "osusergo")):
        raise ValueError("binary must include the shared SQLite/build tags")
    highest, migrations = migration_manifest(ROOT / "backend/migrations")
    info = {
        "format_version": 1, "kind": "server-candidate", "version": version,
        "source_commit": source, "os": "linux", "arch": args.arch,
        "source_date_epoch": int(run("git", "show", "-s", "--format=%ct", "HEAD")),
        "highest_migration": highest, "migrations_sha256": sha256(migrations.encode()),
        "runtime": inspect_binary(binary, args.arch), "go_build": build,
        "node": run("node", "--version"), "pnpm": run("pnpm", "--version"),
        "compiler": run("gcc", "--version"), "libc": run("ldd", "--version"),
        "os_release": Path("/etc/os-release").read_text(),
        "runner": {key: os.environ.get(key) for key in ("ImageOS", "ImageVersion", "RUNNER_ARCH")},
        "input_hashes": {name: sha256((ROOT / name).read_bytes()) for name in
                         ("backend/go.mod", "backend/go.sum", "pnpm-lock.yaml", "package.json")},
        "binary_sha256": sha256(binary.read_bytes()),
    }
    prefix = f"rekenraam-{version}-linux-{args.arch}"
    args.output.mkdir(parents=True, exist_ok=True)
    path = args.output / f"{prefix}.tar.gz"
    write_archive(path, prefix, {
        "rekenraam": (binary.read_bytes(), True),
        "LICENSE": ((ROOT / "LICENSE").read_bytes(), False),
        "BUILDINFO.json": ((json.dumps(info, indent=2, sort_keys=True) + "\n").encode(), False),
        "MIGRATIONS.sha256": (migrations.encode(), False),
    }, info["source_date_epoch"])
    print(path)


def assemble(args):
    version = candidate_version(args.version)
    expected = {f"rekenraam-{version}-linux-{arch}.tar.gz" for arch in ARCHES}
    if {p.name for p in args.input.iterdir()} != expected:
        raise ValueError("assemble requires exactly the amd64 and arm64 archives")
    infos = []
    for arch in ARCHES:
        name = f"rekenraam-{version}-linux-{arch}"
        with tarfile.open(args.input / f"{name}.tar.gz") as archive:
            info = json.load(archive.extractfile(f"{name}/BUILDINFO.json"))
        if info["arch"] != arch or info["version"] != version:
            raise ValueError("archive metadata differs from the requested target")
        infos.append(info)
    for key in ("source_commit", "highest_migration", "migrations_sha256", "input_hashes"):
        if infos[0][key] != infos[1][key]:
            raise ValueError(f"platform archives disagree on {key}")
    if infos[0]["source_commit"] != run("git", "rev-parse", "HEAD"):
        raise ValueError("archives differ from the checked-out source")
    notes = (f"# {version} — server candidate\n\nSource: `{infos[0]['source_commit']}`.\n"
             f"Highest migration: **{infos[0]['highest_migration']}**; all migration hashes "
             "are in each archive's MIGRATIONS.sha256.\n\n"
             "Targets: Linux amd64 and arm64 on Ubuntu 24.04 / glibc 2.39. "
             "BUILDINFO.json records each artifact's actual dependencies and toolchains. "
             "No Windows/Wails desktop artifact is included.\n\n"
             "This candidate is not an approved installed release. Run the release "
             "preflight and retain hosted signing/native-platform evidence. Owner "
             "deployment/household and advisory acceptance remain separate. Before "
             "publishing v0.1.0, freeze every included migration checksum and activate "
             "the installed-release/PR policy. Back up and verify before upgrading; "
             "schema rollback requires restoring the backup and prior binary.\n")
    (args.input / "RELEASE-NOTES.md").write_text(notes)
    files = sorted(args.input.iterdir())
    (args.input / "SHA256SUMS").write_text("".join(
        f"{sha256(p.read_bytes())}  {p.name}\n" for p in files))


def verify_files(directory):
    """Run only AFTER authenticating SHA256SUMS with the expected signer."""
    names = set()
    for line in (directory / "SHA256SUMS").read_text().splitlines():
        match = re.fullmatch(r"([0-9a-f]{64})  ([A-Za-z0-9][A-Za-z0-9._-]*)", line)
        if not match:
            raise ValueError("invalid checksum line or unsafe filename")
        digest, name = match.groups()
        if name in names or name in SIDECARS:
            raise ValueError("duplicate or reserved manifest filename")
        names.add(name)
        path = directory / name
        if path.is_symlink() or not path.is_file() or sha256(path.read_bytes()) != digest:
            raise ValueError(f"checksum mismatch or missing/unsafe file: {name}")
    if not names or {p.name for p in directory.iterdir()} - SIDECARS != names:
        raise ValueError("manifest must cover every download exactly once")
    print(f"Verified {len(names)} release files")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    commands = parser.add_subparsers(dest="command", required=True)
    command = commands.add_parser("package")
    command.add_argument("--version", required=True)
    command.add_argument("--arch", choices=ARCHES, required=True)
    command.add_argument("--binary", type=Path, default=ROOT / "dist/rekenraam")
    command.add_argument("--output", type=Path, required=True)
    command = commands.add_parser("assemble")
    command.add_argument("--version", required=True)
    command.add_argument("--input", type=Path, required=True)
    command = commands.add_parser("verify-files")
    command.add_argument("directory", type=Path)
    command = commands.add_parser("validate-version")
    command.add_argument("version")
    args = parser.parse_args()
    try:
        if args.command == "package":
            package(args)
        elif args.command == "assemble":
            assemble(args)
        elif args.command == "verify-files":
            verify_files(args.directory)
        else:
            print(candidate_version(args.version))
    except (ValueError, OSError, subprocess.CalledProcessError) as error:
        parser.exit(1, f"release artifacts: {error}\n")


if __name__ == "__main__":
    main()

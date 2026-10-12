"""Own the CI text integration database and private bucket for one job invocation."""
import argparse
import json
import os
from pathlib import Path
import re
import signal
import subprocess
import sys
import time
from urllib.parse import urlsplit, urlunsplit
import uuid


class Purpose:
    def __init__(self, journal, fresh=False):
        self.path = Path(journal)
        self.path.parent.mkdir(parents=True, exist_ok=True)
        self.state = {"format": "text-import-purpose-v1", "processes": [], "sticky_preserve": False}
        if fresh:
            if self.path.exists():
                raise RuntimeError("purpose journal already exists; no adoption")
        else:
            self.state = json.loads(self.path.read_text())
            if self.state.get("format") != "text-import-purpose-v1":
                raise RuntimeError("purpose journal format differs")
        if not fresh:
            token = self.state.get("database", "").removeprefix("qvtext_")
            if re.fullmatch(r"[0-9a-f]{32}", token) is None or self.state.get("bucket") != "qv-text-" + token:
                raise RuntimeError("exact journal-owned names differ")
        self.guard = self.path.with_suffix(".pending")
        if self.guard.exists():
            self.state["sticky_preserve"] = True
            self.state["prior_evidence_failure"] = True
        self.cancelled = False
        signal.signal(signal.SIGTERM, self.cancel)
        signal.signal(signal.SIGINT, self.cancel)

    def save(self):
        owns_guard = not self.guard.exists()
        try:
            if owns_guard:
                fd = os.open(self.guard, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
                with os.fdopen(fd, "w") as stream:
                    stream.write("Pending evidence write; preserve if this marker remains.\n")
                    stream.flush()
                    os.fsync(stream.fileno())
            temporary = self.path.with_suffix(".tmp")
            fd = os.open(temporary, os.O_WRONLY | os.O_CREAT | os.O_TRUNC, 0o600)
            with os.fdopen(fd, "w") as stream:
                json.dump(self.state, stream, indent=2)
                stream.write("\n")
                stream.flush()
                os.fsync(stream.fileno())
            os.replace(temporary, self.path)
            if self.cancelled:
                raise RuntimeError("cancellation during evidence write; preserve")
            if owns_guard:
                self.guard.unlink()
        except BaseException as error:
            self.state["sticky_preserve"] = True
            self.state.setdefault("evidence_errors", []).append(type(error).__name__)
            raise

    def cancel(self, number, _frame):
        self.cancelled = True
        self.state["sticky_preserve"] = True
        self.state.setdefault("signal", number)

    def redact(self, value):
        for key, secret in os.environ.items():
            if secret and ("PASSWORD" in key or "SECRET" in key or "DATABASE_URL" in key):
                value = value.replace(secret, "[redacted]")
        return re.sub(r"postgres(?:ql)?://[^\s]+", "[redacted-dsn]", value)

    def command(self, argv, label, env=None, stream=False, timeout=180):
        if self.cancelled or self.state["sticky_preserve"] or self.guard.exists():
            raise RuntimeError("sticky preserve refuses new resource commands")
        entry = {"label": label, "joined": False}
        self.state["processes"].append(entry)
        self.save()
        child = None
        output = ""
        stderr = ""
        primary = None
        try:
            if self.cancelled or self.state["sticky_preserve"]:
                raise RuntimeError("late cancellation before child start; preserve")
            child = subprocess.Popen(argv, env=env, start_new_session=True,
                                     stdout=None if stream else subprocess.PIPE,
                                     stderr=None if stream else subprocess.PIPE, text=True)
            entry["pid"] = child.pid
            self.save()
            deadline = time.monotonic() + timeout
            while True:
                if self.cancelled or time.monotonic() >= deadline:
                    self.state["sticky_preserve"] = True
                    raise RuntimeError("interrupted or timed-out child; preserve")
                try:
                    output, stderr = child.communicate(timeout=1)
                    entry["exit"] = child.returncode
                    if child.returncode:
                        self.state.setdefault("first_failure", {"label": label, "exit": child.returncode})
                    if self.cancelled:
                        self.state["sticky_preserve"] = True
                        raise RuntimeError("late cancellation after communicate; preserve")
                    break
                except subprocess.TimeoutExpired:
                    pass
            if child.returncode:
                primary = RuntimeError(label + " failed with exit " + str(child.returncode))
        except BaseException as error:
            primary = error
            self.state["sticky_preserve"] = True
            if child is not None:
                try:
                    os.killpg(child.pid, signal.SIGTERM)
                except ProcessLookupError:
                    pass
                except BaseException as term_error:
                    entry["term_error"] = type(term_error).__name__
                try:
                    output, stderr = child.communicate(timeout=30)
                except subprocess.TimeoutExpired:
                    entry["unjoined"] = True
        finally:
            if child is None:
                entry["not_started"] = True
                entry["joined"] = True
            else:
                entry["exit"] = child.poll()
                try:
                    os.killpg(child.pid, 0)
                    absent = False
                except ProcessLookupError:
                    absent = True
                except BaseException as group_error:
                    absent = False
                    entry["group_probe_error"] = type(group_error).__name__
                entry["joined"] = child.poll() is not None and absent
                if not entry["joined"]:
                    self.state["sticky_preserve"] = True
                    if primary is None:
                        primary = RuntimeError("child group absence is unproved; preserve")
            if self.cancelled:
                self.state["sticky_preserve"] = True
                if primary is None:
                    primary = RuntimeError("late cancellation after join; preserve")
            if output:
                entry["stdout"] = self.redact(output)
            if stderr:
                entry["stderr"] = self.redact(stderr)
            if primary is not None:
                self.state.setdefault("primary_command_error", self.redact(type(primary).__name__ + ": " + str(primary)))
            try:
                self.save()
            except BaseException as evidence_error:
                if primary is None:
                    primary = evidence_error
        if self.cancelled or self.state["sticky_preserve"]:
            if primary is None:
                primary = RuntimeError("terminal sticky cancellation/evidence failure; preserve")
        if primary is not None:
            raise primary
        return output or ""

    def sql(self, statement, url=None):
        env = os.environ.copy()
        if url is not None:
            env["PGDATABASE"] = url
        raw = self.command(["psql", "-X", "-q", "-A", "-t", "-v", "ON_ERROR_STOP=1", "-d", env.get("PGDATABASE", "postgres"), "-c", statement], "postgres", env)
        return [json.loads(line) for line in raw.splitlines() if line.strip()]

    def catalog(self):
        name = self.state["database"]
        return self.sql("SELECT jsonb_build_object('oid',oid,'owner',pg_get_userbyid(datdba),'connections',(SELECT count(*) FROM pg_stat_activity WHERE datname='" + name + "')) FROM pg_database WHERE datname='" + name + "'")

    def mc(self, args):
        shell = 'mc alias set --quiet local http://minio:9000 "$S3_ACCESS_KEY_ID" "$S3_SECRET_ACCESS_KEY" >/dev/null && exec mc --json "$@"'
        raw = self.command(["docker", "compose", "run", "--rm", "-T", "--no-deps", "--entrypoint", "/bin/sh", "minio-init", "-c", shell, "text-purpose"] + args, "bucket-" + args[0])
        rows = [json.loads(line) for line in raw.splitlines() if line.strip()]
        if any(row.get("status") != "success" for row in rows):
            raise RuntimeError("object-store command reported an error")
        return rows

    def buckets(self):
        rows = self.mc(["ls", "local"])
        if any(row.get("type") != "folder" or not isinstance(row.get("key"), str) for row in rows):
            raise RuntimeError("bucket inventory shape is unknown")
        return [row["key"].rstrip("/") for row in rows]

    def objects(self):
        rows = self.mc(["ls", "--recursive", "--versions", "local/" + self.state["bucket"]])
        if any(row.get("type") != "file" or not isinstance(row.get("key"), str) for row in rows):
            raise RuntimeError("complete object/version inventory shape is unknown")
        return rows

    def prepare(self):
        token = uuid.uuid4().hex
        self.state.update(database="qvtext_" + token, bucket="qv-text-" + token,
                          phase="names_journaled", database_oid=None, bucket_created=False)
        self.save()
        if self.catalog() or self.state["bucket"] in self.buckets():
            raise RuntimeError("fresh purpose absence is not proven")
        self.state["absence_before_create"] = True
        self.save()
        self.sql('CREATE DATABASE "' + self.state["database"] + '" OWNER quizzivy_migrate TEMPLATE template0')
        rows = self.catalog()
        if len(rows) != 1 or rows[0]["owner"] != "quizzivy_migrate" or rows[0]["connections"] != 0:
            raise RuntimeError("created database identity is not proven")
        self.state["database_oid"] = rows[0]["oid"]
        self.save()
        self.state["bucket_creation_requested"] = True
        self.save()
        self.mc(["mb", "local/" + self.state["bucket"]])
        self.state["bucket_created"] = True
        self.save()
        self.mc(["anonymous", "set", "none", "local/" + self.state["bucket"]])
        policy = self.mc(["anonymous", "get", "local/" + self.state["bucket"]])
        if len(policy) != 1 or policy[0].get("status") != "success" or policy[0].get("operation") != "get" or policy[0].get("bucket") != "local/" + self.state["bucket"] or policy[0].get("permission") != "private" or self.objects():
            raise RuntimeError("new private empty bucket is not proven")
        source = Path("migrations")
        versions = sorted(int(path.name.split("_")[0]) for path in source.glob("*.sql"))
        if len(versions) != len(set(versions)) or 91 not in versions:
            raise RuntimeError("source migrations require unique versions including text metadata91")
        base = urlsplit(os.environ["TEST_DATABASE_URL"])
        migrate = urlunsplit(base._replace(path="/" + self.state["database"]))
        app_base = urlsplit(os.environ["TEXT_IMPORT_APP_BASE_URL"])
        app = urlunsplit(app_base._replace(path="/" + self.state["database"]))
        env = os.environ.copy()
        env.update(GOOSE_DRIVER="postgres", GOOSE_DBSTRING=migrate, GOOSE_MIGRATION_DIR="migrations")
        self.command(["goose", "up"], "migrate-text-purpose", env)
        applied = self.sql("SELECT coalesce(jsonb_agg(version_id ORDER BY version_id),'[]'::jsonb) FROM public.goose_db_version WHERE is_applied AND version_id>0", migrate)
        if applied != [versions]:
            raise RuntimeError("actual applied migration chronology differs")
        for url, role in [(app, "quizzivy_app"), (migrate, "quizzivy_migrate")]:
            identity = self.sql("SELECT jsonb_build_object('database',current_database(),'role',current_user,'version',current_setting('server_version_num')::int,'super',rolsuper) FROM pg_roles WHERE rolname=current_user", url)
            if len(identity) != 1 or identity[0]["database"] != self.state["database"] or identity[0]["role"] != role or identity[0]["super"] or identity[0]["version"] < 180000:
                raise RuntimeError("selected ordinary test-role identity is not proven")
            self.state[role] = identity[0]
        acl = self.sql("SELECT jsonb_build_object('audit_update',has_table_privilege(current_user,'app.audit_log','UPDATE'),'audit_delete',has_table_privilege(current_user,'app.audit_log','DELETE'),'events_update',has_table_privilege(current_user,'app.attempt_events','UPDATE'),'events_delete',has_table_privilege(current_user,'app.attempt_events','DELETE'),'runs_update',has_table_privilege(current_user,'app.word_import_run_events','UPDATE'),'runs_delete',has_table_privilege(current_user,'app.word_import_run_events','DELETE'),'commits_update',has_table_privilege(current_user,'app.word_import_commits','UPDATE'),'commits_delete',has_table_privilege(current_user,'app.word_import_commits','DELETE'))", app)
        if len(acl) != 1 or any(acl[0].values()):
            raise RuntimeError("append-only app ACL differs")
        self.state.update(phase="ready", app_append_only_acl=acl[0], migration_versions=versions)
        self.save()
        try:
            with open(os.environ["GITHUB_OUTPUT"], "a") as stream:
                for key, value in [("database", self.state["database"]), ("bucket", self.state["bucket"]), ("app_url", app), ("migrate_url", migrate)]:
                    stream.write(key + "=" + value + "\n")
                stream.flush()
                os.fsync(stream.fileno())
        except BaseException as output_error:
            self.state["sticky_preserve"] = True
            self.state.setdefault("evidence_errors", []).append("github-output-" + type(output_error).__name__)
            raise

    def run(self, argv):
        if self.state.get("phase") != "ready":
            raise RuntimeError("text resources are not ready")
        self.state["integration_started"] = True
        self.save()
        primary = None
        try:
            self.command(argv, "whole-existing-integration", stream=True, timeout=900)
        except BaseException as error:
            primary = error
        finally:
            self.state["integration_joined"] = all(row["joined"] for row in self.state["processes"])
            try:
                self.save()
            except BaseException as evidence_error:
                if primary is None:
                    primary = evidence_error
        if primary is not None:
            raise primary

    def cleanup(self):
        if self.cancelled or self.guard.exists() or self.state["sticky_preserve"] or any(not row["joined"] for row in self.state["processes"]):
            raise RuntimeError("cancelled/unjoined/evidence failure; preserve owned resources")
        if self.state.get("bucket_creation_requested") and not self.state.get("bucket_created"):
            raise RuntimeError("bucket create acknowledgement is unknown; preserve")
        if self.state.get("bucket_created"):
            if self.state["bucket"] not in self.buckets():
                raise RuntimeError("owned bucket unexpectedly missing; preserve")
            inventory = self.objects()
            self.state["owned_versions_before_cleanup"] = inventory
            self.save()
            for row in inventory:
                if row["key"].startswith("/") or ".." in row["key"].split("/") or "\x00" in row["key"]:
                    raise RuntimeError("owned object key is not a safe bucket-relative key")
                args = ["rm"]
                if row.get("versionId"):
                    args += ["--version-id", row["versionId"]]
                self.mc(args + ["local/" + self.state["bucket"] + "/" + row["key"]])
            if self.objects():
                raise RuntimeError("owned bucket is not empty")
            self.state["owned_bucket_empty"] = True
            self.save()
            self.mc(["rb", "local/" + self.state["bucket"]])
            if self.state["bucket"] in self.buckets():
                raise RuntimeError("owned bucket absence is not proven")
            self.state["owned_bucket_absent"] = True
            self.save()
        rows = self.catalog()
        if self.state.get("database_oid") is None:
            if rows:
                raise RuntimeError("database creation identity is unknown; preserve")
        else:
            if rows != [{"oid": self.state["database_oid"], "owner": "quizzivy_migrate", "connections": 0}]:
                raise RuntimeError("exact database identity/zero ALL connections differs; preserve")
            self.state["before_normal_drop"] = rows[0]
            self.save()
            self.sql('DROP DATABASE "' + self.state["database"] + '"')
            if self.catalog():
                raise RuntimeError("normal database DROP absence is not proven")
        self.state.update(phase="disposed", owned_database_absent=True)
        self.save()


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("mode", choices=["prepare", "run", "cleanup"])
    parser.add_argument("journal")
    parser.add_argument("command", nargs=argparse.REMAINDER)
    args = parser.parse_args()
    if args.mode == "cleanup" and not Path(args.journal).exists():
        return 0
    purpose = Purpose(args.journal, fresh=args.mode == "prepare")
    try:
        if purpose.guard.exists():
            raise RuntimeError("prior incomplete evidence write; preserve")
        if args.mode == "prepare":
            purpose.prepare()
        elif args.mode == "cleanup":
            purpose.cleanup()
        else:
            argv = args.command[1:] if args.command[:1] == ["--"] else args.command
            if not argv:
                raise RuntimeError("integration command is missing")
            purpose.run(argv)
    except BaseException as error:
        purpose.state.setdefault("failure", purpose.redact(type(error).__name__ + ": " + str(error)))
        try:
            purpose.save()
        except BaseException:
            print("text purpose evidence write failed; primary failure retained; preserve", file=sys.stderr)
        print("text purpose lifecycle failed; inspect private journal; no forced cleanup", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())

const std = @import("std");
const c = @cImport({
    @cInclude("stdlib.h");
});

pub fn main() !void {
    const allocator = std.heap.page_allocator;
    var args = try std.process.argsAlloc(allocator);
    defer std.process.argsFree(allocator, args);

    if (args.len < 4) {
        std.debug.print("usage: downloader <url> <sessionDir> <workers> [expected_checksum]\n", .{});
        return;
    }

    const url = args[1];
    const session_dir = args[2];
    const workers = try std.fmt.parseInt(usize, args[3], 10);
    const expected_checksum = if (args.len >= 5) args[4] else "";

    const start_time = std.time.timestamp();

    // initialize libcurl via wrapper
    _ = c.curl_global_init_wrapper();
    defer c.curl_global_cleanup_wrapper();

    // quick header fetch using system curl (keeps compatibility)
    var header_path_buf = try std.fmt.allocPrint(allocator, "{s}/_headers", .{session_dir});
    defer allocator.free(header_path_buf);

    var header_cmd = try std.fmt.allocPrint(allocator, "curl -sI -o '{s}' '{s}'", .{header_path_buf, url});
    defer allocator.free(header_cmd);

    var c_header_cmd = try to_c_string(allocator, header_cmd);
    defer allocator.free(c_header_cmd);

    if (c.system(c_header_cmd) != 0) {
        std.debug.print("error: header fetch failed\n", .{});
        return;
    }

    const header_file = try std.fs.cwd().openFile(header_path_buf, .{});
    defer header_file.close();
    const headers = try header_file.readToEndAlloc(allocator, 16 * 1024);
    defer allocator.free(headers);

    var size_bytes: u64 = 0;
    var supports_range: bool = false;
    {
        const lines = std.mem.split(headers, "\n");
        for (lines) |ln| {
            if (std.mem.startsWith(u8, ln, "Content-Length:")) {
                const v = std.mem.trim(u8, ln[15..], " \t\r\n");
                var tmp: []const u8 = v;
                const parsed = std.fmt.parseInt(u64, tmp, 10) catch null;
                if (parsed) |n| size_bytes = n;
            }
            if (std.mem.indexOf(u8, ln, "Accept-Ranges:") != null) {
                if (std.mem.indexOf(u8, ln, "bytes") != null) supports_range = true;
            }
        }
    }

    if (size_bytes == 0) {
        std.debug.print("error: could not determine Content-Length\n", .{});
        return;
    }

    var actual_workers = workers;
    if (!supports_range) actual_workers = 1;

    const chunk = size_bytes / actual_workers;

    const Part = struct {
        start: u64,
        end: u64,
        id: usize,
    };

    var parts = try allocator.alloc(Part, actual_workers);
    for (actual_workers) |i| {
        const s = @intCast(u64, i) * chunk;
        var e = s + chunk - 1;
        if (i == actual_workers - 1) e = size_bytes - 1;
        parts[i] = Part{ .start = s, .end = e, .id = i };
    }

    const WorkerCtx = struct {
        part: *Part,
        url: []const u8,
        session_dir: []const u8,
        max_retries: u32,
        allocator: *std.mem.Allocator,
    };

    var ctxs = try allocator.alloc(WorkerCtx, actual_workers);
    var threads = try allocator.alloc(std.Thread, actual_workers);

    for (actual_workers) |i| {
        ctxs[i] = WorkerCtx{ .part = &parts[i], .url = url, .session_dir = session_dir, .max_retries = 3, .allocator = allocator };
        threads[i] = try std.Thread.spawn(.{}, worker, &ctxs[i]);
    }

    for (actual_workers) |i| {
        _ = threads[i].wait();
    }

    // validate parts
    var all_ok = true;
    for (actual_workers) |i| {
        const p = parts[i];
        const part_path = try std.fmt.allocPrint(allocator, "{s}/part_{d}.tmp", .{session_dir, p.id});
        defer allocator.free(part_path);
        const fi = std.fs.cwd().stat(part_path) catch null;
        const expected = p.end - p.start + 1;
        if (fi == null) { all_ok = false; break; }
        if (fi.*.size != expected) { all_ok = false; break; }
    }

    if (!all_ok) {
        std.debug.print("error: some parts failed to download\n", .{});
        return;
    }

    const filename = base_name_from_url(url, allocator) orelse "downloaded.file";
    const dest_path = try std.fmt.allocPrint(allocator, "{s}/{s}", .{session_dir, filename});
    defer allocator.free(dest_path);

    const dest_file = try std.fs.cwd().createFile(dest_path, .{ .create = true, .truncate = true });
    defer dest_file.close();

    var copy_buf: [64 * 1024]u8 = undefined;
    for (actual_workers) |i| {
        const p = parts[i];
        const part_path = try std.fmt.allocPrint(allocator, "{s}/part_{d}.tmp", .{session_dir, p.id});
        defer allocator.free(part_path);
        const pf = try std.fs.cwd().openFile(part_path, .{});
        defer pf.close();
        while (true) {
            const n = try pf.read(copy_buf[0..]);
            if (n == 0) break;
            try dest_file.writeAll(copy_buf[0..n]);
        }
        try std.fs.cwd().remove(part_path);
    }

    const end_time = std.time.timestamp();
    const duration_ns = end_time - start_time;
    const duration_s = @intToFloat(f64, duration_ns) / 1_000_000_000.0;
    const avg_mbps = (@intCast(f64, size_bytes) / (1024.0 * 1024.0)) / duration_s;

    const computed_buf = try compute_sha256(dest_path, allocator);
    const computed = computed_buf;

    const log_path = try std.fmt.allocPrint(allocator, "{s}/downloader.log", .{session_dir});
    defer allocator.free(log_path);
    const lf = try std.fs.cwd().createFile(log_path, .{ .create = true, .truncate = true });
    defer lf.close();
    try lf.writeAllFmt("START_TIME: {d}\n", .{start_time});
    try lf.writeAllFmt("END_TIME: {d}\n", .{end_time});
    try lf.writeAllFmt("DURATION_SECONDS: {s}\n", .{@floatToString(f64, duration_s)});
    try lf.writeAllFmt("AVERAGE_SPEED_MBPS: {s}\n", .{@floatToString(f64, avg_mbps)});
    try lf.writeAllFmt("WORKERS: {d}\n", .{actual_workers});
    try lf.writeAllFmt("FILE: {s}\n", .{filename});
    try lf.writeAllFmt("SIZE_BYTES: {d}\n", .{size_bytes});
    try lf.writeAllFmt("CHECKSUM: {s}\n", .{computed});
    if (expected_checksum.len > 0) {
        const match = std.mem.eql(u8, computed, expected_checksum);
        try lf.writeAllFmt("CHECKSUM_MATCH: {s}\n", .{ if (match) "true" else "false" });
        if (!match) {
            try lf.writeAll("STATUS: FAIL\n");
            try lf.writeAll("MESSAGE: checksum mismatch\n");
            std.debug.print("checksum mismatch\n", .{});
            return;
        }
    }
    try lf.writeAll("STATUS: OK\n");

    std.debug.print("download complete: {s} ({d} bytes)\n", .{filename, size_bytes});
}

fn to_c_string(allocator: *std.mem.Allocator, s: []const u8) ![*]const u8 {
    const len = s.len;
    const buf = try allocator.alloc(u8, len + 1);
    std.mem.copy(u8, buf[0..len], s);
    buf[len] = 0;
    return @ptrCast([*]const u8, buf.ptr);
}

fn worker(ctx_ptr: ?*c_void) void {
    const ctx = @ptrCast(*WorkerCtx, ctx_ptr);
    const allocator = ctx.allocator;
    const p = ctx.part.*;
    const part_path = tryOrPanic(allocator, std.fmt.allocPrint(allocator, "{s}/part_{d}.tmp", .{ctx.session_dir, p.id}));
    defer allocator.free(part_path);

    var attempt: u32 = 0;
    while (attempt < ctx.max_retries) {
        var existing: u64 = 0;
        const st = std.fs.cwd().stat(part_path) catch null;
        if (st) existing = st.*.size;
        const range_start = p.start + existing;
        if (range_start > p.end) return;

        var bytes_received: c.longlong = 0;

        const cmd = tryOrPanic(allocator, std.fmt.allocPrint(allocator, "curl --fail -s --location --range {d}-{d} '{s}' >> '{s}'", .{range_start, p.end, ctx.url, part_path}));
        const ccmd = tryOrPanic(allocator, to_c_string(allocator, cmd));
        const rc = c.system(ccmd);
        allocator.free(ccmd);
        allocator.free(cmd);
        if (rc == 0) return;
        attempt += 1;
        std.time.sleep(std.time.milliSecond(500 * attempt));
    }
}

fn tryOrPanic(allocator: *std.mem.Allocator, s: []const u8) []const u8 {
    if (s.len == 0) @panic("allocation failed");
    return s;
}

fn compute_sha256(path: []const u8, allocator: *std.mem.Allocator) ![]u8 {
    var file = try std.fs.cwd().openFile(path, .{});
    defer file.close();
    var hasher = std.crypto.hash.sha2.Sha256.init(.{});
    var buf: [64 * 1024]u8 = undefined;
    while (true) {
        const n = try file.read(buf[0..]);
        if (n == 0) break;
        hasher.update(buf[0..n]);
    }
    var digest: [32]u8 = undefined;
    hasher.final(&digest);
    var out = try allocator.alloc(u8, 64 + 1);
    var pos: usize = 0;
    for (digest) |b| {
        const hex = std.fmt.formatIntHex(b, .lower);
        const hexBytes = hex;
        std.mem.copy(u8, out[pos..pos + hexBytes.len], hexBytes);
        pos += hexBytes.len;
    }
    out[pos] = 0;
    return out[0..pos];
}

fn base_name_from_url(url: []const u8, allocator: *std.mem.Allocator) ?[]const u8 {
    var idx: isize = -1;
    for (url) |c, i| {
        if (c == '/') idx = i;
    }
    if (idx < 0) return null;
    const start = @intCast(usize, idx + 1);
    if (start >= url.len) return null;
    return url[start..];
}

const std = @import("std");

pub fn main() !void {
    var gpa = std.heap.GeneralPurposeAllocator(.{}){};
    defer _ = gpa.deinit();
    const allocator = gpa.allocator();

    var args = try std.process.argsAlloc(allocator);
    defer std.process.argsFree(allocator, args);

    if (args.len < 3) {
        std.debug.print("usage: verify <file> <expected_hash>\n", .{});
        return;
    }

    const path = args[1];
    const expected_hash = args[2];

    const file = try std.fs.cwd().openFile(path, .{});
    defer file.close();

    const data = try file.readToEndAlloc(allocator, 1 * 1024 * 1024 * 1024);
    defer allocator.free(data);

    var hasher = std.crypto.hash.sha2.Sha256.init(.{});
    hasher.update(data);
    var digest: [32]u8 = undefined;
    hasher.final(&digest);

    var computed_hash: [64]u8 = undefined;
    for (digest, 0..) |byte, i| {
        _ = try std.fmt.bufPrint(computed_hash[i * 2 .. i * 2 + 2], "{x:0>2}", .{byte});
    }

    if (std.mem.eql(u8, &computed_hash, expected_hash)) {
        std.debug.print("verified:true\n", .{});
    } else {
        std.debug.print("verified:false\n", .{});
    }
}

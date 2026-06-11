const std = @import("std");

pub fn main() !void {
    var gpa = std.heap.GeneralPurposeAllocator(.{}){};
    defer _ = gpa.deinit();
    const allocator = gpa.allocator();

    var args = try std.process.argsAlloc(allocator);
    defer std.process.argsFree(allocator, args);

    if (args.len < 3) {
        std.debug.print("usage: compare <file1> <file2>\n", .{});
        return;
    }

    const path1 = args[1];
    const path2 = args[2];

    const file1 = try std.fs.cwd().openFile(path1, .{});
    defer file1.close();

    const file2 = try std.fs.cwd().openFile(path2, .{});
    defer file2.close();

    const data1 = try file1.readToEndAlloc(allocator, 1 * 1024 * 1024 * 1024);
    defer allocator.free(data1);

    const data2 = try file2.readToEndAlloc(allocator, 1 * 1024 * 1024 * 1024);
    defer allocator.free(data2);

    if (std.mem.eql(u8, data1, data2)) {
        std.debug.print("match:true\n", .{});
    } else {
        std.debug.print("match:false\n", .{});
    }
}

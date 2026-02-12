use clap::Parser;
use std::env;

#[derive(Parser, Debug)]
#[command(version, about, long_about=None )]
struct Args {
    #[arg(short, long)]
    project: String,

    #[arg(short, long)]
    item: i32,
}

fn main() {
    let args = Args::parse();
}

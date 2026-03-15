use std::{env, io};

fn main() {
    let mut out = io::stdout();
    let mut err = io::stderr();
    let working_dir = env::current_dir().unwrap_or_else(|_| env::temp_dir());
    let args = env::args().skip(1).collect::<Vec<_>>();
    let refs = args.iter().map(String::as_str).collect::<Vec<_>>();
    let exit = mutate4rs::run(&refs, working_dir, &mut out, &mut err);
    std::process::exit(exit);
}

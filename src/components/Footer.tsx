import React from 'react';
import { Film, Github, Twitter, Instagram, Mail } from 'lucide-react';
import { clsx, type ClassValue } from 'clsx';
import { twMerge } from 'tailwind-merge';

function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}

const Footer = () => {
  const currentYear = new Date().getFullYear();

  return (
    <footer className='bg-dark text-gray-400 py-12 border-t border-gray-800'>
      <div className='max-w-7xl mx-auto px-6 flex flex-col md:flex-row justify-between items-center gap-8'>
        <div className='flex flex-col items-center md:items-start gap-4'>
          <div className='flex items-center gap-2 text-primary font-bold text-xl tracking-tighter text-white'>
            <Film size={24} />
            <span>YASH</span>
          </div>
          <p className='text-sm text-center md:text-left max-w-xs'>
            Crafting cinematic digital experiences with passion and precision.
          </p>
        </div>

        <div className='flex gap-6'>
          <a href='#' className='hover:text-primary transition-colors' aria-label='GitHub'>
            <Github size={20} />
          </a>
          <a href='#' className='hover:text-primary transition-colors' aria-label='Twitter'>
            <Twitter size={20} />
          </a>
          <a href='#' className='hover:text-primary transition-colors' aria-label='Instagram'>
            <Instagram size={20} />
          </a>
          <a href='#' className='hover:text-primary transition-colors' aria-label='Email'>
            <Mail size={20} />
          </a>
        </div>

        <div className='text-sm'>
          © {currentYear} Yash Cinematic. All rights reserved.
        </div>
      </div>
    </footer>
  );
};

export default Footer;
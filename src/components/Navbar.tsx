import React, { useState, useEffect } from 'react';
import { motion } from 'framer-motion';
import { Menu, X, Film } from 'lucide-react';
import { clsx, type ClassValue } from 'clsx';
import { twMerge } from 'tailwind-merge';

// Utility for tailwind class merging
function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}

const Navbar = () => {
  const [isScrolled, setIsScrolled] = useState(false);
  const [isMobileMenuOpen, setIsMobileMenuOpen] = useState(false);

  useEffect(() => {
    const handleScroll = () => {
      setIsScrolled(window.scrollY > 50);
    };

    window.addEventListener('scroll', handleScroll);
    return () => window.removeEventListener('scroll', handleScroll);
  }, []);

  const navLinks = [
    { name: 'Home', href: '#home' },
    { name: 'Work', href: '#work' },
    { name: 'About', href: '#about' },
    { name: 'Contact', href: '#contact' },
  ];

  return (
    <motion.nav
      initial={{ y: -100 }}
      animate={{ y: 0 }}
      className={cn(
        'fixed top-0 left-0 right-0 z-50 transition-colors duration-300 px-6 py-4',
        isScrolled ? 'bg-secondary shadow-md' : 'bg-transparent'
      )}
    >
      <div className='max-w-7xl mx-auto flex justify-between items-center'>
        <div className='flex items-center gap-2 text-primary font-bold text-2xl tracking-tighter'>
          <Film size={28} />
          <span>YASH</span>
        </div>

        {/* Desktop Navigation */}
        <div className='hidden md:flex items-center gap-8'>
          {navLinks.map((link) => (
            <a
              key={link.name}
              href={link.href}
              className='text-gray-300 hover:text-white transition-colors text-sm font-medium uppercase tracking-widest'
            >
              {link.name}
            </a>
          ))}
          <button className='bg-primary text-white px-4 py-2 rounded text-sm font-bold hover:bg-red-700 transition-colors'>
            Hire Me
          </button>
        </div>

        {/* Mobile Menu Button */}
        <div className='md:hidden flex items-center'>
          <button 
            onClick={() => setIsMobileMenuOpen(!isMobileMenuOpen)}
            className='text-white focus:outline-none'
          >
            {isMobileMenuOpen ? <X size={28} /> : <Menu size={28} />}
          </button>
        </div>
      </div>

      {/* Mobile Navigation Overlay */}
      {isMobileMenuOpen && (
        <motion.div
          initial={{ opacity: 0, height: 0 }}
          animate={{ opacity: 1, height: 'auto' }}
          exit={{ opacity: 0, height: 0 }}
          className='md:hidden bg-secondary absolute top-full left-0 right-0 border-t border-gray-800 overflow-hidden'
        >
          <div className='flex flex-col p-6 gap-4'>
            {navLinks.map((link) => (
              <a
                key={link.name}
                href={link.href}
                onClick={() => setIsMobileMenuOpen(false)}
                className='text-gray-300 hover:text-white transition-colors text-lg font-medium'
              >
                {link.name}
              </a>
            ))}
            <button className='bg-primary text-white px-4 py-3 rounded text-center font-bold'>
              Hire Me
            </button>
          </div>
        </motion.div>
      )}
    </motion.nav>
  );
};

export default Navbar;